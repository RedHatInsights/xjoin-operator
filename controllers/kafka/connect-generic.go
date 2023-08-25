package kafka

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"text/template"
	"time"
)

func (kafka *GenericKafka) CreateGenericDebeziumConnector(
	name string, namespace string, connectorTemplate string, connectorTemplateParameters map[string]interface{}, dryRun bool) (unstructured.Unstructured, error) {

	connectorObj := &unstructured.Unstructured{}
	connectorConfig, err := kafka.parseConnectorTemplate(connectorTemplate, connectorTemplateParameters)
	if err != nil {
		return *connectorObj, errors.Wrap(err, 0)
	}
	connectorObj.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]interface{}{
				LabelStrimziCluster: kafka.ConnectCluster,
			},
		},
		"spec": map[string]interface{}{
			"class":    "io.debezium.connector.postgresql.PostgresConnector",
			"config":   connectorConfig,
			"pause":    false,
			"tasksMax": 1,
		},
	}

	connectorObj.SetGroupVersionKind(connectorGVK)

	if dryRun {
		return *connectorObj, nil
	}

	err = kafka.Client.Create(kafka.Context, connectorObj)
	if err != nil {
		return *connectorObj, errors.Wrap(err, 0)
	}

	return *connectorObj, nil
}

func (kafka *GenericKafka) CreateGenericElasticsearchConnector(
	name string, namespace string, connectorTemplate string, connectorTemplateParameters map[string]interface{}, dryRun bool) (unstructured.Unstructured, error) {

	connectorObj := &unstructured.Unstructured{}
	connectorConfig, err := kafka.parseConnectorTemplate(connectorTemplate, connectorTemplateParameters)
	if err != nil {
		return *connectorObj, errors.Wrap(err, 0)
	}

	connectorObj.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]interface{}{
				LabelStrimziCluster: kafka.ConnectCluster,
			},
		},
		"spec": map[string]interface{}{
			"class":    "io.confluent.connect.elasticsearch.ElasticsearchSinkConnector",
			"config":   connectorConfig,
			"pause":    false,
			"tasksMax": connectorTemplateParameters["ElasticSearchTasksMax"],
		},
	}

	connectorObj.SetGroupVersionKind(connectorGVK)

	if dryRun {
		return *connectorObj, nil
	}

	err = kafka.Client.Create(kafka.Context, connectorObj)
	if err != nil {
		return *connectorObj, errors.Wrap(err, 0)
	}

	return *connectorObj, nil
}

func (kafka *GenericKafka) parseConnectorTemplate(connectorTemplate string, connectorTemplateParameters map[string]interface{}) (interface{}, error) {
	tmpl, err := template.New("configTemplate").Parse(connectorTemplate)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	var configTemplateBuffer bytes.Buffer
	err = tmpl.Execute(&configTemplateBuffer, connectorTemplateParameters)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	configTemplateParsed := configTemplateBuffer.String()

	var configTemplateInterface interface{}

	err = json.Unmarshal([]byte(strings.ReplaceAll(configTemplateParsed, "\n", "")), &configTemplateInterface)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return configTemplateInterface, nil
}

func (kafka *GenericKafka) DeleteConnector(name string) error {
	if name == "" {
		return nil
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()

	connector := &unstructured.Unstructured{}
	connector.SetName(name)
	connector.SetNamespace(kafka.ConnectNamespace)
	connector.SetGroupVersionKind(connectorGVK)

	//check the Connect REST API every second for 10 seconds to see if the connector is really deleted.
	//This is necessary because there is a race condition in Kafka Connect. If a connector
	//and topic is deleted in rapid succession, the Kafka Connect tasks get stuck trying to connect to a topic
	//that doesn't exist.
	delay := time.Millisecond * 100
	attempts := 200
	connectorIsDeleted := false
	missingCount := 0
	for i := 0; i < attempts; i++ {
		if err := kafka.Client.Delete(ctx, connector); err != nil && !k8errors.IsNotFound(err) {
			return errors.Wrap(err, 0)
		}

		connectorExists, err := kafka.CheckConnectorExistsViaREST(name)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		if !connectorExists {
			missingCount = missingCount + 1
		}

		if missingCount > 5 {
			connectorIsDeleted = true
			break
		}

		time.Sleep(delay)
	}

	if !connectorIsDeleted {
		return errors.Wrap(errors.New(fmt.Sprintf("connector %s wasn't deleted after 10 seconds", name)), 0)
	}

	return nil
}

func (kafka *GenericKafka) CheckConnectorExistsViaREST(name string) (bool, error) {
	url := fmt.Sprintf(
		"%s/connectors/%s",
		kafka.ConnectUrl(), name)

	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	res, err := httpClient.Do(req)

	if err != nil {
		return false, err
	}

	if res.StatusCode == 404 {
		return false, nil
	} else if res.StatusCode < 500 {
		return true, nil
	} else {
		return false, errors.New(fmt.Sprintf(
			"invalid response code (%s) when checking if connector %v exists", name, res.StatusCode))
	}
}

func (kafka *GenericKafka) ConnectUrl() string {
	url := fmt.Sprintf(
		"http://%s-connect-api.%s.svc:8083",
		kafka.ConnectCluster, kafka.ConnectNamespace)
	return url
}

func (kafka *GenericKafka) CheckIfConnectorExists(name string, namespace string) (bool, error) {
	if name == "" {
		return false, nil
	}

	if _, err := kafka.GetConnector(name, namespace); err != nil && k8errors.IsNotFound(err) {
		return false, nil
	} else if err == nil {
		return true, nil
	} else {
		return false, errors.Wrap(err, 0)
	}
}

func (kafka *GenericKafka) GetConnector(name string, namespace string) (*unstructured.Unstructured, error) {
	connector := EmptyConnector()
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err := kafka.Client.Get(
		ctx,
		client.ObjectKey{Name: name, Namespace: namespace},
		connector)
	return connector, err
}

func (kafka *GenericKafka) ListConnectorNamesForPrefix(prefix string) ([]string, error) {
	kafka.Log.Debug("Listing connectors for prefix",
		"namespace", kafka.ConnectNamespace, "prefix", prefix)

	connectors, err := kafka.ListConnectors()
	if err != nil {
		return nil, err
	}

	var names []string
	for _, connector := range connectors.Items {
		kafka.Log.Debug("ListConnectorNamesForPrefix connector", "name", connector.GetName())

		if strings.Index(connector.GetName(), prefix) == 0 {
			names = append(names, connector.GetName())
		}
	}

	return names, err
}

func (kafka *GenericKafka) ListConnectors() (*unstructured.UnstructuredList, error) {
	connectors := &unstructured.UnstructuredList{}
	connectors.SetGroupVersionKind(connectorsGVK)

	err := kafka.Client.List(kafka.Context, connectors, client.InNamespace(kafka.ConnectNamespace))
	return connectors, err
}
