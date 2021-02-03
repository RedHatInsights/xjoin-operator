package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"strings"
	"text/template"

	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func (kafka *Kafka) CheckIfConnectorExists(name string) (bool, error) {
	if name == "" {
		return false, nil
	}

	if _, err := kafka.GetConnector(name); err != nil && k8errors.IsNotFound(err) {
		return false, nil
	} else if err == nil {
		return true, nil
	} else {
		return false, err
	}
}

func (kafka *Kafka) newESConnectorResource(pipelineVersion string) (*unstructured.Unstructured, error) {
	m := kafka.ParametersMap
	m["Version"] = pipelineVersion

	return kafka.newConnectorResource(
		kafka.ESConnectorName(pipelineVersion),
		"io.confluent.connect.elasticsearch.ElasticsearchSinkConnector",
		m,
		kafka.Parameters.ElasticSearchConnectorTemplate.String())
}

func (kafka *Kafka) newDebeziumConnectorResource(pipelineVersion string) (*unstructured.Unstructured, error) {
	m := kafka.ParametersMap
	m["Version"] = pipelineVersion
	m["ReplicationSlotName"] = database.ReplicationSlotName(kafka.Parameters.ResourceNamePrefix.String(), pipelineVersion)

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
			"namespace": kafka.Parameters.ConnectClusterNamespace.String(),
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

func (kafka *Kafka) GetConnector(name string) (*unstructured.Unstructured, error) {
	connector := EmptyConnector()
	err := kafka.Client.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: kafka.Parameters.ConnectClusterNamespace.String()}, connector)
	return connector, err
}

func (kafka *Kafka) DeleteConnectorsForPipelineVersion(pipelineVersion string) error {
	connectorsToDelete, err := kafka.ListConnectorNamesForPipelineVersion(pipelineVersion)
	if err != nil {
		return err
	}

	for _, connector := range connectorsToDelete {
		err = kafka.DeleteConnector(connector)
		if err != nil {
			return err
		}
	}

	return nil
}

func (kafka *Kafka) ListConnectorNamesForPipelineVersion(pipelineVersion string) ([]string, error) {
	connectors, err := kafka.ListConnectors()
	if err != nil {
		return nil, err
	}

	var names []string
	for _, connector := range connectors.Items {
		if strings.Index(connector.GetName(), pipelineVersion) != -1 {
			names = append(names, connector.GetName())
		}
	}

	return names, err
}

func (kafka *Kafka) ListConnectors() (*unstructured.UnstructuredList, error) {
	connectors := &unstructured.UnstructuredList{}
	connectors.SetGroupVersionKind(connectorsGVK)

	err := kafka.Client.List(context.TODO(), connectors, client.InNamespace(kafka.Parameters.ConnectClusterNamespace.String()))
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
func (kafka *Kafka) DeleteConnector(name string) error {
	connector := &unstructured.Unstructured{}
	connector.SetName(name)
	connector.SetNamespace(kafka.Parameters.ConnectClusterNamespace.String())
	connector.SetGroupVersionKind(connectorGVK)

	if err := kafka.Client.Delete(context.TODO(), connector); err != nil && !k8errors.IsNotFound(err) {
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

func (kafka *Kafka) PauseElasticSearchConnector(pipelineVersion string) error {
	return kafka.setElasticSearchConnectorPause(pipelineVersion, true)
}

func (kafka *Kafka) ResumeElasticSearchConnector(pipelineVersion string) error {
	return kafka.setElasticSearchConnectorPause(pipelineVersion, false)
}

func (kafka *Kafka) setElasticSearchConnectorPause(pipelineVersion string, pause bool) error {
	connector, err := kafka.GetConnector(kafka.ESConnectorName(pipelineVersion))
	if err != nil {
		return err
	}

	connector.Object["spec"].(map[string]interface{})["pause"] = pause

	err = kafka.Client.Update(context.TODO(), connector)
	if err != nil {
		return err
	}

	return nil
}

func (kafka *Kafka) CreateDryConnectorByType(conType string, version string) (*unstructured.Unstructured, error) {
	if conType == "es" {
		return kafka.CreateESConnector(version, true)
	} else if conType == "debezium" {
		return kafka.CreateDebeziumConnector(version, true)
	} else {
		return nil, errors.New("invalid param. Must be one of [es, debezium]")
	}
}
