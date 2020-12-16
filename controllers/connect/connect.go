package connect

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	LabelAppName        = "cyndi/appName"
	LabelInsightsOnly   = "cyndi/insightsOnly"
	LabelMaxAge         = "cyndi/maxAge"
	LabelStrimziCluster = "strimzi.io/cluster"
	LabelOwner          = "cyndi/owner"
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
	Cluster     string
	Template    string
	HBIDBParams config.DBParams
	Version     string
	MaxAge      int64
}

type ElasticSearchConnectorConfiguration struct {
	Cluster               string
	Template              string
	ElasticSearchURL      string
	ElasticSearchUsername string
	ElasticSearchPassword string
	Version               string
	MaxAge                int64
}

func CheckIfConnectorExists(c client.Client, name string, namespace string) (bool, error) {
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

func newESConnectorResource(
	namespace string,
	config ElasticSearchConnectorConfiguration) (*unstructured.Unstructured, error) {
	m := make(map[string]string)
	m["ElasticSearchURL"] = config.ElasticSearchURL
	m["ElasticSearchUsername"] = config.ElasticSearchUsername
	m["ElasticSearchPassword"] = config.ElasticSearchPassword
	m["Version"] = config.Version

	return newConnectorResource(
		v1alpha1.ESConnectorName(config.Version),
		namespace,
		config.Cluster,
		"io.confluent.connect.elasticsearch.ElasticsearchSinkConnector",
		m,
		config.Template)
}

func newDebeziumConnectorResource(
	namespace string,
	config DebeziumConnectorConfiguration) (*unstructured.Unstructured, error) {

	m := make(map[string]string)
	m["DBPort"] = config.HBIDBParams.Port
	m["DBHostname"] = config.HBIDBParams.Host
	m["DBName"] = config.HBIDBParams.Name
	m["DBUser"] = config.HBIDBParams.User
	m["DBPassword"] = config.HBIDBParams.Password
	m["Version"] = config.Version

	return newConnectorResource(
		v1alpha1.DebeziumConnectorName(config.Version),
		namespace,
		config.Cluster,
		"io.debezium.connector.postgresql.PostgresConnector",
		m,
		config.Template)
}

func newConnectorResource(name string,
	namespace string,
	cluster string,
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
			"namespace": namespace,
			"labels": map[string]interface{}{
				LabelStrimziCluster: cluster,
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

func CreateESConnector(c client.Client,
	namespace string,
	config ElasticSearchConnectorConfiguration,
	owner metav1.Object,
	ownerScheme *runtime.Scheme,
	dryRun bool) (*unstructured.Unstructured, error) {
	connector, err := newESConnectorResource(namespace, config)

	if err != nil {
		return nil, err
	}

	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, connector, ownerScheme); err != nil {
			return nil, err
		}

		labels := connector.GetLabels()
		labels[LabelOwner] = string(owner.GetUID())
		connector.SetLabels(labels)
	}

	if dryRun {
		return connector, nil
	}

	return connector, c.Create(context.TODO(), connector)
}

func CreateDebeziumConnector(
	c client.Client,
	namespace string,
	config DebeziumConnectorConfiguration,
	owner metav1.Object,
	ownerScheme *runtime.Scheme,
	dryRun bool) (*unstructured.Unstructured, error) {

	connector, err := newDebeziumConnectorResource(namespace, config)

	if err != nil {
		return nil, err
	}

	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, connector, ownerScheme); err != nil {
			return nil, err
		}

		labels := connector.GetLabels()
		labels[LabelOwner] = string(owner.GetUID())
		connector.SetLabels(labels)
	}

	if dryRun {
		return connector, nil
	}

	return connector, c.Create(context.TODO(), connector)
}
