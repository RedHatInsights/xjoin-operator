package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"net/http"
	"strings"
	"text/template"
	"time"

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
	Version: "v1beta2",
}

var connectorsGVK = schema.GroupVersionKind{
	Group:   "kafka.strimzi.io",
	Kind:    "KafkaConnector",
	Version: "v1beta2",
}

func (c *StrimziConnectors) newESConnectorResource(pipelineVersion string) (*unstructured.Unstructured, error) {
	m := c.Kafka.ParametersMap
	m["Topic"] = c.Topics.TopicName(pipelineVersion)
	m["RenameTopicReplacement"] = fmt.Sprintf("%s.%s", c.Kafka.Parameters.ResourceNamePrefix.String(), pipelineVersion)

	return c.newConnectorResource(
		c.ESConnectorName(pipelineVersion),
		"io.confluent.connect.elasticsearch.ElasticsearchSinkConnector",
		m,
		c.Kafka.Parameters.ElasticSearchConnectorTemplate.String())
}

func (c *StrimziConnectors) newDebeziumConnectorResource(pipelineVersion string) (*unstructured.Unstructured, error) {
	m := c.Kafka.ParametersMap
	m["Version"] = pipelineVersion
	m["ReplicationSlotName"] = database.ReplicationSlotName(c.Kafka.Parameters.ResourceNamePrefix.String(), pipelineVersion)

	return c.newConnectorResource(
		c.DebeziumConnectorName(pipelineVersion),
		"io.debezium.connector.postgresql.PostgresConnector",
		m,
		c.Kafka.Parameters.DebeziumTemplate.String())
}

func (c *StrimziConnectors) newConnectorResource(
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
			"namespace": c.Kafka.Parameters.ConnectClusterNamespace.String(),
			"labels": map[string]interface{}{
				LabelStrimziCluster:    c.Kafka.Parameters.ConnectCluster.String(),
				"resource.name.prefix": c.Kafka.Parameters.ResourceNamePrefix.String(),
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

func (c *StrimziConnectors) GetTaskStatus(connectorName string, taskId float64) (map[string]interface{}, error) {
	url := fmt.Sprintf(
		"%s/connectors/%s/tasks/%.0f/status",
		c.Kafka.ConnectUrl(), connectorName, taskId)

	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := httpClient.Do(req)

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

func (c *StrimziConnectors) ListConnectorTasks(connectorName string) ([]map[string]interface{}, error) {
	connectorStatus, err := c.GetConnectorStatus(connectorName)
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

func (c *StrimziConnectors) RestartTaskForConnector(connectorName string, taskId float64) error {
	metrics.ConnectorTaskRestarted(connectorName)
	log.Warn("Restarting connector task", "connector", connectorName, "taskId", taskId)

	url := fmt.Sprintf(
		"%s/connectors/%s/tasks/%.0f/restart",
		c.Kafka.ConnectUrl(),
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

func (c *StrimziConnectors) verifyTaskIsRunning(task map[string]interface{}, connectorName string) (bool, error) {
	//try to restart the task 10 times
	for i := 0; i < 10; i++ {
		if task["state"] == running {
			return true, nil
		} else {
			err := c.RestartTaskForConnector(connectorName, task["id"].(float64))

			if err != nil {
				return false, err
			}

			time.Sleep(2 * time.Second)

			task, err = c.GetTaskStatus(connectorName, task["id"].(float64))
		}
	}

	return false, nil //restarts failed
}

func (c *StrimziConnectors) RestartConnector(connectorName string) error {
	tasks, err := c.ListConnectorTasks(connectorName)
	if err != nil {
		return err
	}

	for _, task := range tasks {
		taskIsValid, err := c.verifyTaskIsRunning(task, connectorName)
		if err != nil {
			return err
		} else if !taskIsValid {
			return errors.New("connector is in failed state")
		}
	}

	return nil
}

// CheckIfConnectIsResponding
// First validate /connectors responds. If there are existing connectors, validate one of those can be retrieved too.
func (c *StrimziConnectors) CheckIfConnectIsResponding() (bool, error) {
	for j := 0; j < 10; j++ {
		url := c.Kafka.ConnectUrl() + "/connectors"

		httpClient := &http.Client{Timeout: 15 * time.Second}
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return false, err
		}
		res, err := httpClient.Do(req)

		//ignore errors and try again
		if err != nil {
			log.Info("Error checking if connect is responding, trying again.", "error", err)
		}

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
				req2, err := http.NewRequest(http.MethodGet, connectorUrl, nil)
				if err != nil {
					return false, err
				}
				res2, _ := httpClient.Do(req2)

				if res2 != nil && res2.StatusCode == 200 {
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

func (c *StrimziConnectors) ListConnectorsREST(prefix string) ([]string, error) {
	url := fmt.Sprintf(
		"%s/connectors",
		c.Kafka.ConnectUrl())

	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := httpClient.Do(req)

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var connectors []string
	if res.StatusCode == 200 {
		body, err := ioutil.ReadAll(res.Body)
		err = json.Unmarshal(body, &connectors)
		if err != nil {
			return nil, err
		}
	} else if res.StatusCode != 404 {
		return nil, errors.New("unable to get connectors")
	}

	var connectorsFiltered []string
	for _, connector := range connectors {
		if strings.Index(connector, prefix) == 0 {
			connectorsFiltered = append(connectorsFiltered, connector)
		}
	}

	return connectorsFiltered, nil
}

func (c *StrimziConnectors) DeleteConnectorsForPipelineVersion(pipelineVersion string) error {
	connectorsToDelete, err := c.ListConnectorNamesForPipelineVersion(pipelineVersion)
	if err != nil {
		return err
	}

	for _, connector := range connectorsToDelete {
		err = c.Kafka.DeleteConnector(connector)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *StrimziConnectors) ListConnectorNamesForPipelineVersion(pipelineVersion string) ([]string, error) {
	connectors, err := c.Kafka.ListConnectors()
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

func EmptyConnector() *unstructured.Unstructured {
	connector := &unstructured.Unstructured{}
	connector.SetGroupVersionKind(connectorGVK)
	return connector
}

func (c *StrimziConnectors) DeleteAllConnectors(resourceNamePrefix string) error {
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	connector := &unstructured.Unstructured{}
	connector.SetGroupVersionKind(connectorsGVK)

	err := c.Kafka.Client.DeleteAllOf(
		ctx,
		connector,
		client.InNamespace(c.Kafka.Parameters.KafkaClusterNamespace.String()),
		client.MatchingLabels{"resource.name.prefix": resourceNamePrefix})

	if err != nil {
		return err
	}

	log.Info("Waiting for connectors to be deleted")
	err = wait.PollImmediate(time.Second, time.Duration(300)*time.Second, func() (bool, error) {
		connectors, err := c.ListConnectorsREST(resourceNamePrefix)
		if err != nil {
			return false, err
		}
		if len(connectors) > 0 {
			return false, nil
		} else {
			return true, nil
		}
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *StrimziConnectors) GetConnectorStatus(connectorName string) (map[string]interface{}, error) {
	log.Info("Getting connector status for connector" + connectorName)
	url := fmt.Sprintf(
		"%s/connectors/%s/status",
		c.Kafka.ConnectUrl(), connectorName)
	log.Info("connector status url: " + url)

	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := httpClient.Do(req)

	if err != nil {
		return nil, err
	}
	log.Info("got status for url: " + url)
	defer res.Body.Close()

	var bodyMap map[string]interface{}
	if res.StatusCode == 200 {
		log.Info("Successfully got status for url: " + url)
		body, err := ioutil.ReadAll(res.Body)
		err = json.Unmarshal(body, &bodyMap)
		if err != nil {
			return nil, err
		}
	} else if res.StatusCode != 404 {
		log.Info("error getting connector status at url: " + url)
		return nil, errors.New(fmt.Sprintf("unable to get connector %s status", connectorName))
	}

	log.Info("connector not found at url: " + url)

	return bodyMap, nil
}

func (c *StrimziConnectors) IsFailed(connectorName string) (bool, error) {
	connectorStatus, err := c.GetConnectorStatus(connectorName)
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

func (c *StrimziConnectors) CreateESConnector(
	pipelineVersion string,
	dryRun bool) (*unstructured.Unstructured, error) {

	connector, err := c.newESConnectorResource(pipelineVersion)
	if err != nil {
		return nil, err
	}

	if dryRun {
		return connector, nil
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	return connector, c.Kafka.Client.Create(ctx, connector)
}

func (c *StrimziConnectors) CreateDebeziumConnector(
	pipelineVersion string,
	dryRun bool) (*unstructured.Unstructured, error) {

	connector, err := c.newDebeziumConnectorResource(pipelineVersion)
	if err != nil {
		return nil, err
	}
	if dryRun {
		return connector, nil
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	return connector, c.Kafka.Client.Create(ctx, connector)
}

func (c *StrimziConnectors) DebeziumConnectorName(pipelineVersion string) string {
	return fmt.Sprintf("%s.db.%s", c.Kafka.Parameters.ResourceNamePrefix.String(), pipelineVersion)
}

func (c *StrimziConnectors) ESConnectorName(pipelineVersion string) string {
	return fmt.Sprintf("%s.es.%s", c.Kafka.Parameters.ResourceNamePrefix.String(), pipelineVersion)
}

func (c *StrimziConnectors) PauseElasticSearchConnector(pipelineVersion string) error {
	return c.setElasticSearchConnectorPause(pipelineVersion, true)
}

func (c *StrimziConnectors) ResumeElasticSearchConnector(pipelineVersion string) error {
	return c.setElasticSearchConnectorPause(pipelineVersion, false)
}

func (c *StrimziConnectors) setElasticSearchConnectorPause(pipelineVersion string, pause bool) error {
	connector, err := c.Kafka.GetConnector(c.ESConnectorName(pipelineVersion))
	if err != nil {
		return err
	}

	connector.Object["spec"].(map[string]interface{})["pause"] = pause

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = c.Kafka.Client.Update(ctx, connector)
	if err != nil {
		return err
	}

	return nil
}

func (c *StrimziConnectors) CreateDryConnectorByType(conType string, version string) (*unstructured.Unstructured, error) {
	if conType == "es" {
		return c.CreateESConnector(version, true)
	} else if conType == "debezium" {
		return c.CreateDebeziumConnector(version, true)
	} else {
		return nil, errors.New("invalid param. Must be one of [es, debezium]")
	}
}

func (c *StrimziConnectors) updateConnectDepReplicas(newReplicas int64) (currentReplicas int64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*999)
	defer cancel()

	//try a few times in case the deployment is updated between the GET/PUT which causes the update to fail
	for j := 0; j < 10; j++ {
		//retrieve deployment object
		var deploymentGVK = schema.GroupVersionKind{
			Group:   "apps",
			Kind:    "Deployment",
			Version: "v1",
		}

		deployment := &unstructured.Unstructured{}
		deployment.SetGroupVersionKind(deploymentGVK)
		err = c.Kafka.Client.Get(
			ctx,
			client.ObjectKey{Name: c.Kafka.Parameters.ConnectCluster.String() + "-connect", Namespace: c.Kafka.Parameters.ConnectClusterNamespace.String()},
			deployment)
		if err != nil {
			return
		}

		//update deployment object
		obj := deployment.Object
		spec := make(map[string]interface{})
		if obj["spec"] != nil {
			spec = obj["spec"].(map[string]interface{})
		}
		currentReplicas = spec["replicas"].(int64)
		spec["replicas"] = newReplicas
		err = c.Kafka.Client.Update(ctx, deployment)
		if err == nil {
			break
		}
	}
	return
}

func (c *StrimziConnectors) RestartConnect() error {
	metrics.ConnectRestarted()
	log.Warn("Restarting Kafka Connect")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*999)
	defer cancel()

	//get current pod template hash
	pods := &corev1.PodList{}

	labels := client.MatchingLabels{}
	labels["app.kubernetes.io/part-of"] = "strimzi-" + c.Kafka.Parameters.ConnectCluster.String()
	err := c.Kafka.Client.List(ctx, pods, labels)
	if err != nil {
		return err
	}

	if len(pods.Items) < 1 {
		return errors.New("no Kafka Connect instance found")
	}

	//podLabels := pods.Items[0].GetLabels()
	//currentHash := podLabels["pod-template-hash"]

	currentReplicas, err := c.updateConnectDepReplicas(0)
	if err != nil {
		return err
	}
	_, err = c.updateConnectDepReplicas(currentReplicas)
	if err != nil {
		return err
	}

	log.Info("Waiting for Kafka Connect to be ready...")
	//wait for deployment to complete by checking each pods status
	err = wait.PollImmediate(time.Second*time.Duration(2), time.Duration(1800)*time.Second, func() (bool, error) {
		ctx, cancel = utils.DefaultContext()
		defer cancel()

		var replicasetGVK = schema.GroupVersionKind{
			Group:   "apps",
			Kind:    "ReplicaSet",
			Version: "v1",
		}

		replicaset := &unstructured.UnstructuredList{}
		replicaset.SetGroupVersionKind(replicasetGVK)
		err = c.Kafka.Client.List(ctx, pods, labels)

		labels := client.MatchingLabels{}
		labels["app.kubernetes.io/part-of"] = "strimzi-" + c.Kafka.Parameters.ConnectCluster.String()
		err = c.Kafka.Client.List(ctx, replicaset, labels)
		status := replicaset.Items[0].Object["status"].(map[string]interface{})
		replicas := status["replicas"]
		readyReplicas := status["readyReplicas"]

		if replicas != readyReplicas {
			return false, nil
		} else {
			return true, nil
		}

		/*
			pods := &corev1.PodList{}

			ctx, cancel = utils.DefaultContext()
			defer cancel()

			labels := Client.MatchingLabels{}
			labels["app.kubernetes.io/part-of"] = "strimzi-" + c.Parameters.ConnectCluster.String()
			err = c.Client.List(ctx, pods, labels)

			//only check pods for new deployment
			var newPods []corev1.Pod

			for _, pod := range pods.Items {
				podLabels := pod.GetLabels()
				if podLabels["pod-template-hash"] != currentHash {
					newPods = append(newPods, pod)
				}
			}

			if len(newPods) == 0 {
				return false, nil
			}

			//check the status of each pod for the new deployment
			for _, pod := range pods.Items {
				for _, condition := range pod.Status.Conditions {
					if condition.Type == "Ready" {
						if condition.Status != "True" {
							return false, nil
						}
					}
				}
			}
			return true, nil

		*/
	})

	if err != nil {
		return err
	}

	log.Info("Kafka Connect successfully restarted")
	return nil
}
