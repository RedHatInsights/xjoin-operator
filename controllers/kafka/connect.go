package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	LabelStrimziCluster = "strimzi.io/cluster"
	LabelOwner          = "xjoin/owner"
)

const failed = "FAILED"

var connectorGVK = schema.GroupVersionKind{
	Group:   "kafka.strimzi.io",
	Kind:    "KafkaConnector",
	Version: "v1alpha1",
}

var connectorsGVK = schema.GroupVersionKind{
	Group:   "kafka.strimzi.io",
	Kind:    "KafkaConnectorList",
	Version: "v1alpha1",
}

func (kafka *Kafka) CheckIfConnectorExists(c client.Client, name string, namespace string) (bool, error) {
	if name == "" {
		return false, nil
	}

	if _, err := GetConnector(c, name, namespace); err != nil && errors.IsNotFound(err) {
		return false, nil
	} else if err == nil {
		return true, nil
	} else {
		return false, err
	}
}

func (kafka *Kafka) newESConnectorResource(
	config config.Parameters,
	pipelineVersion string) (*unstructured.Unstructured, error) {

	m := make(map[string]interface{})
	m["ElasticSearchURL"] = config.ElasticSearchURL.String()
	m["ElasticSearchUsername"] = config.ElasticSearchUsername.String()
	m["ElasticSearchPassword"] = config.ElasticSearchPassword.String()
	m["Version"] = pipelineVersion
	m["TasksMax"] = config.ElasticSearchTasksMax.Int()
	m["MaxInFlightRequests"] = config.ElasticSearchMaxInFlightRequests.Int()
	m["ErrorsLogEnable"] = config.ElasticSearchErrorsLogEnable.Bool()
	m["MaxRetries"] = config.ElasticSearchMaxRetries.Int()
	m["RetryBackoffMS"] = config.ElasticSearchRetryBackoffMS.Int()
	m["BatchSize"] = config.ElasticSearchBatchSize.Int()
	m["MaxBufferedRecords"] = config.ElasticSearchMaxBufferedRecords.Int()
	m["LingerMS"] = config.ElasticSearchLingerMS.Int()

	return kafka.newConnectorResource(
		ESConnectorName(config.ResourceNamePrefix.String(), pipelineVersion),
		"io.confluent.connect.elasticsearch.ElasticsearchSinkConnector",
		m,
		config.ElasticSearchConnectorTemplate.String())
}

func (kafka *Kafka) newDebeziumConnectorResource(
	config config.Parameters,
	pipelineVersion string) (*unstructured.Unstructured, error) {

	m := make(map[string]interface{})
	m["DBPort"] = config.HBIDBPort.String()
	m["DBHostname"] = config.HBIDBHost.String()
	m["DBName"] = config.HBIDBName.String()
	m["DBUser"] = config.HBIDBUser.String()
	m["DBPassword"] = config.HBIDBPassword.String()
	m["Version"] = pipelineVersion
	m["TasksMax"] = config.DebeziumTasksMax.Int()
	m["MaxBatchSize"] = config.DebeziumMaxBatchSize.Int()
	m["QueueSize"] = config.DebeziumQueueSize.Int()
	m["PollIntervalMS"] = config.DebeziumPollIntervalMS.Int()
	m["ErrorsLogEnable"] = config.DebeziumErrorsLogEnable.Bool()

	return kafka.newConnectorResource(
		DebeziumConnectorName(config.ResourceNamePrefix.String(), pipelineVersion),
		"io.debezium.connector.postgresql.PostgresConnector",
		m,
		config.DebeziumTemplate.String())
}

func (kafka *Kafka) newConnectorResource(
	name string,
	class string,
	connectorConfig map[string]interface{},
	connectorTemplate string) (*unstructured.Unstructured, error) {

	tmpl, err := template.New("configTemplate").Parse(connectorTemplate)
	if err != nil {
		return nil, err
	}

	var configTemplateBuffer bytes.Buffer
	err = tmpl.Execute(&configTemplateBuffer, connectorConfig)
	if err != nil {
		return nil, err
	}
	configTemplateParsed := configTemplateBuffer.String()

	var configTemplateInterface interface{}

	err = json.Unmarshal([]byte(strings.ReplaceAll(configTemplateParsed, "\n", "")), &configTemplateInterface)
	if err != nil {
		return nil, err
	}

	u := &unstructured.Unstructured{}
	u.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": kafka.Namespace,
			"labels": map[string]interface{}{
				LabelStrimziCluster: kafka.Parameters.ConnectCluster.String(),
			},
		},
		"spec": map[string]interface{}{
			"class":  class,
			"config": configTemplateInterface,
			"pause":  false,
		},
	}

	u.SetGroupVersionKind(connectorGVK)
	return u, nil
}

func GetConnector(c client.Client, name string, namespace string) (*unstructured.Unstructured, error) {
	connector := EmptyConnector()
	err := c.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: namespace}, connector)
	return connector, err
}

func GetConnectorsForOwner(c client.Client, namespace string, owner string) (*unstructured.UnstructuredList, error) {
	connectors := &unstructured.UnstructuredList{}
	connectors.SetGroupVersionKind(connectorsGVK)

	err := c.List(context.TODO(), connectors, client.InNamespace(namespace), client.MatchingLabels{LabelOwner: owner})
	return connectors, err
}

func EmptyConnector() *unstructured.Unstructured {
	connector := &unstructured.Unstructured{}
	connector.SetGroupVersionKind(connectorGVK)
	return connector
}

/*
 * Delete the given connector. This operation is idempotent i.e. it silently ignores if the connector does not exist.
 */
func DeleteConnector(c client.Client, name string, namespace string) error {
	connector := &unstructured.Unstructured{}
	connector.SetName(name)
	connector.SetNamespace(namespace)
	connector.SetGroupVersionKind(connectorGVK)

	if err := c.Delete(context.TODO(), connector); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func IsFailed(connector *unstructured.Unstructured) bool {
	connectorStatus, ok, err := unstructured.NestedString(connector.UnstructuredContent(), "status", "connectorStatus", "connector", "state")

	if err == nil && ok && connectorStatus == failed {
		return true
	}

	tasks, ok, err := unstructured.NestedSlice(connector.UnstructuredContent(), "status", "connectorStatus", "tasks")

	if ok && err == nil {
		for _, task := range tasks {
			taskMap, ok := task.(map[string]interface{})

			if ok && taskMap["state"] == failed {
				return true
			}
		}
	}

	return false
}

func (kafka *Kafka) CreateESConnector(
	config config.Parameters,
	pipelineVersion string,
	dryRun bool) (*unstructured.Unstructured, error) {

	connector, err := kafka.newESConnectorResource(config, pipelineVersion)
	if err != nil {
		return nil, err
	}

	if kafka.Owner != nil {
		if err := controllerutil.SetControllerReference(kafka.Owner, connector, kafka.OwnerScheme); err != nil {
			return nil, err
		}

		labels := connector.GetLabels()
		labels[LabelOwner] = string(kafka.Owner.GetUID())
		connector.SetLabels(labels)
	}

	if dryRun {
		return connector, nil
	}

	return connector, kafka.Client.Create(context.TODO(), connector)
}

func (kafka *Kafka) CreateDebeziumConnector(
	config config.Parameters,
	pipelineVersion string,
	dryRun bool) (*unstructured.Unstructured, error) {

	connector, err := kafka.newDebeziumConnectorResource(config, pipelineVersion)
	if err != nil {
		return nil, err
	}

	if kafka.Owner != nil {
		if err := controllerutil.SetControllerReference(kafka.Owner, connector, kafka.OwnerScheme); err != nil {
			return nil, err
		}

		labels := connector.GetLabels()
		labels[LabelOwner] = string(kafka.Owner.GetUID())
		connector.SetLabels(labels)
	}

	if dryRun {
		return connector, nil
	}

	return connector, kafka.Client.Create(context.TODO(), connector)
}

func DebeziumConnectorName(name string, pipelineVersion string) string {
	return fmt.Sprintf("%s.db.%s", name, pipelineVersion)
}

func ESConnectorName(name string, pipelineVersion string) string {
	return fmt.Sprintf("%s.es.%s", name, pipelineVersion)
}
