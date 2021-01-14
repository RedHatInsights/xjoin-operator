package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func (kafka *Kafka) newESConnectorResource(pipelineVersion string) (*unstructured.Unstructured, error) {

	m := make(map[string]interface{})
	m["ElasticSearchURL"] = kafka.Parameters.ElasticSearchURL.String()
	m["ElasticSearchUsername"] = kafka.Parameters.ElasticSearchUsername.String()
	m["ElasticSearchPassword"] = kafka.Parameters.ElasticSearchPassword.String()
	m["Version"] = pipelineVersion
	m["TasksMax"] = kafka.Parameters.ElasticSearchTasksMax.Int()
	m["MaxInFlightRequests"] = kafka.Parameters.ElasticSearchMaxInFlightRequests.Int()
	m["ErrorsLogEnable"] = kafka.Parameters.ElasticSearchErrorsLogEnable.Bool()
	m["MaxRetries"] = kafka.Parameters.ElasticSearchMaxRetries.Int()
	m["RetryBackoffMS"] = kafka.Parameters.ElasticSearchRetryBackoffMS.Int()
	m["BatchSize"] = kafka.Parameters.ElasticSearchBatchSize.Int()
	m["MaxBufferedRecords"] = kafka.Parameters.ElasticSearchMaxBufferedRecords.Int()
	m["LingerMS"] = kafka.Parameters.ElasticSearchLingerMS.Int()

	return kafka.newConnectorResource(
		kafka.ESConnectorName(pipelineVersion),
		"io.confluent.connect.elasticsearch.ElasticsearchSinkConnector",
		m,
		kafka.Parameters.ElasticSearchConnectorTemplate.String())
}

func (kafka *Kafka) newDebeziumConnectorResource(pipelineVersion string) (*unstructured.Unstructured, error) {

	m := make(map[string]interface{})
	m["DBPort"] = kafka.Parameters.HBIDBPort.String()
	m["DBHostname"] = kafka.Parameters.HBIDBHost.String()
	m["DBName"] = kafka.Parameters.HBIDBName.String()
	m["DBUser"] = kafka.Parameters.HBIDBUser.String()
	m["DBPassword"] = kafka.Parameters.HBIDBPassword.String()
	m["Version"] = pipelineVersion
	m["TasksMax"] = kafka.Parameters.DebeziumTasksMax.Int()
	m["MaxBatchSize"] = kafka.Parameters.DebeziumMaxBatchSize.Int()
	m["QueueSize"] = kafka.Parameters.DebeziumQueueSize.Int()
	m["PollIntervalMS"] = kafka.Parameters.DebeziumPollIntervalMS.Int()
	m["ErrorsLogEnable"] = kafka.Parameters.DebeziumErrorsLogEnable.Bool()

	return kafka.newConnectorResource(
		kafka.DebeziumConnectorName(pipelineVersion),
		"io.debezium.connector.postgresql.PostgresConnector",
		m,
		kafka.Parameters.DebeziumTemplate.String())
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

func (kafka *Kafka) ListConnectors() (*unstructured.UnstructuredList, error) {
	connectors := &unstructured.UnstructuredList{}
	connectors.SetGroupVersionKind(connectorsGVK)

	err := kafka.Client.List(context.TODO(), connectors, client.InNamespace(kafka.Namespace))
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
	pipelineVersion string,
	dryRun bool) (*unstructured.Unstructured, error) {

	connector, err := kafka.newESConnectorResource(pipelineVersion)
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
	pipelineVersion string,
	dryRun bool) (*unstructured.Unstructured, error) {

	connector, err := kafka.newDebeziumConnectorResource(pipelineVersion)
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

func (kafka *Kafka) DebeziumConnectorName(pipelineVersion string) string {
	return fmt.Sprintf("%s.db.%s", kafka.Parameters.ResourceNamePrefix.String(), pipelineVersion)
}

func (kafka *Kafka) ESConnectorName(pipelineVersion string) string {
	return fmt.Sprintf("%s.es.%s", kafka.Parameters.ResourceNamePrefix.String(), pipelineVersion)
}
