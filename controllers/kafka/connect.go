package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	"io/ioutil"
	"net/http"
	"strings"
	"text/template"
	"time"

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
const running = "RUNNING"

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
	m["Topic"] = kafka.TopicName(pipelineVersion)
	m["RenameTopicReplacement"] = fmt.Sprintf("%s.%s", kafka.Parameters.ResourceNamePrefix.String(), pipelineVersion)

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

func (kafka *Kafka) GetTaskStatus(connectorName string, taskId float64) (map[string]interface{}, error) {
	url := fmt.Sprintf(
		"http://%s-connect-api.%s.svc:8083/connectors/%s/tasks/%.0f/status",
		kafka.Parameters.ConnectCluster.String(), kafka.Parameters.ConnectClusterNamespace.String(), connectorName, taskId)
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	var bodyMap map[string]interface{}
	err = json.Unmarshal(body, &bodyMap)
	if err != nil {
		return nil, err
	}
	return bodyMap, nil
}

func (kafka *Kafka) ListConnectorTasks(connectorName string) ([]map[string]interface{}, error) {
	connectorStatus, err := kafka.GetConnectorStatus(connectorName)
	if err != nil {
		return nil, err
	}

	var tasksMap []map[string]interface{}
	tasks := connectorStatus["tasks"]

	if tasks != nil {
		tasksInterface := tasks.([]interface{})
		for _, task := range tasksInterface {
			taskMap := task.(map[string]interface{})
			tasksMap = append(tasksMap, taskMap)
		}
	}
	return tasksMap, nil
}

func (kafka *Kafka) RestartTaskForConnector(connectorName string, taskId float64) error {
	metrics.ConnectorTaskRestarted()
	log.Warn("Restarting connector task", "connector", connectorName, "taskId", taskId)

	url := fmt.Sprintf(
		"http://%s-connect-api.%s.svc:8083/connectors/%s/tasks/%.0f/restart",
		kafka.Parameters.ConnectCluster.String(),
		kafka.Parameters.ConnectClusterNamespace.String(),
		connectorName,
		taskId)
	res, err := http.Post(url, "application/json", nil)

	if err != nil {
		return err
	}

	if res.StatusCode >= 300 {
		return errors.New("invalid response from connect when restarting task")
	}

	return nil
}

func (kafka *Kafka) verifyTaskIsRunning(task map[string]interface{}, connectorName string) (bool, error) {
	//try to restart the task 10 times
	for i := 0; i < 10; i++ {
		if task["state"] == running {
			return true, nil
		} else {
			err := kafka.RestartTaskForConnector(connectorName, task["id"].(float64))

			if err != nil {
				return false, err
			}

			time.Sleep(1 * time.Second)

			task, err = kafka.GetTaskStatus(connectorName, task["id"].(float64))
		}
	}

	return false, nil //restarts failed
}

func (kafka *Kafka) RestartConnector(connectorName string) error {
	tasks, err := kafka.ListConnectorTasks(connectorName)
	if err != nil {
		return err
	}

	for _, task := range tasks {
		taskIsValid, err := kafka.verifyTaskIsRunning(task, connectorName)
		if err != nil {
			return err
		} else if !taskIsValid {
			return errors.New("connector is in failed state")
		}
	}

	return nil
}

func (kafka *Kafka) GetConnector(name string) (*unstructured.Unstructured, error) {
	connector := EmptyConnector()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := kafka.Client.Get(
		ctx,
		client.ObjectKey{Name: name, Namespace: kafka.Parameters.ConnectClusterNamespace.String()},
		connector)
	return connector, err
}

// CheckIfConnectIsResponding
// First validate /connectors responds. If there are existing connectors, validate one of those can be retrieved too.
func (kafka *Kafka) CheckIfConnectIsResponding() (bool, error) {
	for j := 0; j < 10; j++ {
		url := fmt.Sprintf(
			"http://%s-connect-api.%s.svc:8083/connectors",
			kafka.Parameters.ConnectCluster.String(), kafka.Parameters.ConnectClusterNamespace.String())
		res, _ := http.Get(url) //ignore errors and try again

		if res != nil && res.StatusCode == 200 {
			body, err := ioutil.ReadAll(res.Body)
			err = res.Body.Close()
			if err != nil {
				return false, nil
			}

			var bodyArr []string
			err = json.Unmarshal(body, &bodyArr)
			if err != nil {
				return false, err
			}

			//if there are connectors, make sure they can be retrieved
			if len(bodyArr) > 0 {
				connectorUrl := url + "/" + bodyArr[0]
				res, _ = http.Get(connectorUrl) //ignore errors and try again

				if res != nil && res.StatusCode == 200 {
					return true, nil
				}
			} else {
				//there are no connectors so signal connect is up
				return true, nil
			}
		}
	}

	log.Warn("Kafka Connect is not responding")
	return false, nil
}

func (kafka *Kafka) CheckConnectorExistsViaREST(name string) (bool, error) {
	url := fmt.Sprintf(
		"http://%s-connect-api.%s.svc:8083/connectors/%s",
		kafka.Parameters.ConnectCluster.String(), kafka.Parameters.ConnectClusterNamespace.String(), name)
	res, err := http.Get(url)
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

func (kafka *Kafka) ListConnectorNamesForPrefix(prefix string) ([]string, error) {
	connectors, err := kafka.ListConnectors()
	if err != nil {
		return nil, err
	}

	var names []string
	for _, connector := range connectors.Items {
		if strings.Index(connector.GetName(), prefix) == 0 {
			names = append(names, connector.GetName())
		}
	}

	return names, err
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := kafka.Client.List(ctx, connectors, client.InNamespace(kafka.Parameters.ConnectClusterNamespace.String()))
	return connectors, err
}

func EmptyConnector() *unstructured.Unstructured {
	connector := &unstructured.Unstructured{}
	connector.SetGroupVersionKind(connectorGVK)
	return connector
}

func (kafka *Kafka) DeleteConnector(name string) error {
	if name == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()

	connector := &unstructured.Unstructured{}
	connector.SetName(name)
	connector.SetNamespace(kafka.Parameters.ConnectClusterNamespace.String())
	connector.SetGroupVersionKind(connectorGVK)

	//check the Connect REST API every second for 10 seconds to see if the connector is really deleted.
	//This is necessary because there is a race condition in Kafka Connect. If a connector
	//and topic is deleted in rapid succession, the Kafka Connect tasks get stuck trying to connect to a topic
	//that doesn't exist.
	delay := time.Millisecond * 50
	attempts := 200
	connectorIsDeleted := false
	for i := 0; i < attempts; i++ {
		if err := kafka.Client.Delete(ctx, connector); err != nil && !k8errors.IsNotFound(err) {
			return err
		}

		connectorExists, err := kafka.CheckConnectorExistsViaREST(name)
		if err != nil {
			return err
		}

		if !connectorExists {
			connectorIsDeleted = true
			break
		} else {
			time.Sleep(delay)
		}
	}

	if !connectorIsDeleted {
		return errors.New(fmt.Sprintf("connector %s wasn't deleted after 10 seconds", name))
	}

	return nil
}

func (kafka *Kafka) GetConnectorStatus(connectorName string) (map[string]interface{}, error) {
	url := fmt.Sprintf(
		"http://%s-connect-api.%s.svc:8083/connectors/%s/status",
		kafka.Parameters.ConnectCluster.String(), kafka.Parameters.ConnectClusterNamespace.String(), connectorName)
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var bodyMap map[string]interface{}
	if res.StatusCode == 200 {
		body, err := ioutil.ReadAll(res.Body)
		err = json.Unmarshal(body, &bodyMap)
		if err != nil {
			return nil, err
		}
	} else if res.StatusCode != 404 {
		return nil, errors.New(fmt.Sprintf("unable to get connector %s status", connectorName))
	}

	return bodyMap, nil
}

func (kafka *Kafka) IsFailed(connectorName string) (bool, error) {
	connectorStatus, err := kafka.GetConnectorStatus(connectorName)
	if err != nil {
		return false, err
	}

	var connector interface{}
	connector = connectorStatus["connector"]
	if connector != nil {
		connectorMap := connectorStatus["connector"].(map[string]interface{})
		connectorState := connectorMap["state"].(string)

		if connectorState == failed {
			return true, nil
		}
	}

	tasks := connectorStatus["tasks"]
	if tasks != nil {
		tasksInterface := tasks.([]interface{})

		var tasksMap []map[string]interface{}
		for _, task := range tasksInterface {
			taskMap := task.(map[string]interface{})
			tasksMap = append(tasksMap, taskMap)
		}

		for _, task := range tasksMap {
			if task["state"] == failed {
				return true, nil
			}
		}
	}

	return false, nil
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	return connector, kafka.Client.Create(ctx, connector)
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	return connector, kafka.Client.Create(ctx, connector)
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err = kafka.Client.Update(ctx, connector)
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
