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
	LabelMaxAge         = "xjoin/maxAge"
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

type DebeziumConnectorConfiguration struct {
	Template           string
	HBIDBParams        config.DBParams
	Version            string
	MaxAge             int64
	ResourceNamePrefix string
}

type ElasticSearchConnectorConfiguration struct {
	Template              string
	ElasticSearchURL      string
	ElasticSearchUsername string
	ElasticSearchPassword string
	Version               string
	MaxAge                int64
	ResourceNamePrefix    string
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

func (kafka *Kafka) newESConnectorResource(config ElasticSearchConnectorConfiguration) (*unstructured.Unstructured, error) {
	m := make(map[string]string)
	m["ElasticSearchURL"] = config.ElasticSearchURL
	m["ElasticSearchUsername"] = config.ElasticSearchUsername
	m["ElasticSearchPassword"] = config.ElasticSearchPassword
	m["Version"] = config.Version

	return kafka.newConnectorResource(
		ESConnectorName(config.ResourceNamePrefix, config.Version),
		"io.confluent.connect.elasticsearch.ElasticsearchSinkConnector",
		m,
		config.Template)
}

func (kafka *Kafka) newDebeziumConnectorResource(
	config DebeziumConnectorConfiguration) (*unstructured.Unstructured, error) {

	m := make(map[string]string)
	m["DBPort"] = config.HBIDBParams.Port
	m["DBHostname"] = config.HBIDBParams.Host
	m["DBName"] = config.HBIDBParams.Name
	m["DBUser"] = config.HBIDBParams.User
	m["DBPassword"] = config.HBIDBParams.Password
	m["Version"] = config.Version

	return kafka.newConnectorResource(
		DebeziumConnectorName(config.ResourceNamePrefix, config.Version),
		"io.debezium.connector.postgresql.PostgresConnector",
		m,
		config.Template)
}

func (kafka *Kafka) newConnectorResource(
	name string,
	class string,
	connectorConfig map[string]string,
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
				LabelStrimziCluster: kafka.ConnectCluster,
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

// TODO move to k8s?
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
	config ElasticSearchConnectorConfiguration,
	dryRun bool) (*unstructured.Unstructured, error) {

	connector, err := kafka.newESConnectorResource(config)
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
	config DebeziumConnectorConfiguration,
	dryRun bool) (*unstructured.Unstructured, error) {

	connector, err := kafka.newDebeziumConnectorResource(config)
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
