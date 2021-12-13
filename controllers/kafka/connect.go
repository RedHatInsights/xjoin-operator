package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
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
				LabelStrimziCluster:    kafka.Parameters.ConnectCluster.String(),
				"resource.name.prefix": kafka.Parameters.ResourceNamePrefix.String(),
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
		"%s/connectors/%s/tasks/%.0f/status",
		kafka.ConnectUrl(), connectorName, taskId)
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
	metrics.ConnectorTaskRestarted(connectorName)
	log.Warn("Restarting connector task", "connector", connectorName, "taskId", taskId)

	url := fmt.Sprintf(
		"%s/connectors/%s/tasks/%.0f/restart",
		kafka.ConnectUrl(),
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

			time.Sleep(2 * time.Second)

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

// CheckIfConnectIsResponding
// First validate /connectors responds. If there are existing connectors, validate one of those can be retrieved too.
func (kafka *Kafka) CheckIfConnectIsResponding() (bool, error) {
	for j := 0; j < 10; j++ {
		url := kafka.ConnectUrl() + "/connectors"
		res, err := http.Get(url)

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

func (kafka *Kafka) ListConnectorsREST(prefix string) ([]string, error) {
	url := fmt.Sprintf(
		"%s/connectors",
		kafka.ConnectUrl())
	res, err := http.Get(url)
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

func EmptyConnector() *unstructured.Unstructured {
	connector := &unstructured.Unstructured{}
	connector.SetGroupVersionKind(connectorGVK)
	return connector
}

func (kafka *Kafka) DeleteAllConnectors(resourceNamePrefix string) error {
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	connector := &unstructured.Unstructured{}
	connector.SetGroupVersionKind(connectorsGVK)

	err := kafka.Client.DeleteAllOf(
		ctx,
		connector,
		client.InNamespace(kafka.Parameters.KafkaClusterNamespace.String()),
		client.MatchingLabels{"resource.name.prefix": resourceNamePrefix})

	if err != nil {
		return err
	}

	log.Info("Waiting for connectors to be deleted")
	err = wait.PollImmediate(time.Second, time.Duration(300)*time.Second, func() (bool, error) {
		connectors, err := kafka.ListConnectorsREST(resourceNamePrefix)
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

func (kafka *Kafka) GetConnectorStatus(connectorName string) (map[string]interface{}, error) {
	log.Info("Getting connector status for connector" + connectorName)
	url := fmt.Sprintf(
		"%s/connectors/%s/status",
		kafka.ConnectUrl(), connectorName)
	log.Info("connector status url: " + url)
	res, err := http.Get(url)
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

	ctx, cancel := utils.DefaultContext()
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

	ctx, cancel := utils.DefaultContext()
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

	ctx, cancel := utils.DefaultContext()
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

func (kafka *Kafka) updateConnectDepReplicas(newReplicas int64) (currentReplicas int64, err error) {
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
		err = kafka.Client.Get(
			ctx,
			client.ObjectKey{Name: kafka.Parameters.ConnectCluster.String() + "-connect", Namespace: kafka.Parameters.ConnectClusterNamespace.String()},
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
		err = kafka.Client.Update(ctx, deployment)
		if err == nil {
			break
		}
	}
	return
}

func (kafka *Kafka) RestartConnect() error {
	metrics.ConnectRestarted()
	log.Warn("Restarting Kafka Connect")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*999)
	defer cancel()

	//get current pod template hash
	pods := &corev1.PodList{}

	labels := client.MatchingLabels{}
	labels["app.kubernetes.io/part-of"] = "strimzi-" + kafka.Parameters.ConnectCluster.String()
	err := kafka.Client.List(ctx, pods, labels)
	if err != nil {
		return err
	}

	if len(pods.Items) < 1 {
		return errors.New("no Kafka Connect instance found")
	}

	//podLabels := pods.Items[0].GetLabels()
	//currentHash := podLabels["pod-template-hash"]

	currentReplicas, err := kafka.updateConnectDepReplicas(0)
	if err != nil {
		return err
	}
	_, err = kafka.updateConnectDepReplicas(currentReplicas)
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
		err = kafka.Client.List(ctx, pods, labels)

		labels := client.MatchingLabels{}
		labels["app.kubernetes.io/part-of"] = "strimzi-" + kafka.Parameters.ConnectCluster.String()
		err = kafka.Client.List(ctx, replicaset, labels)
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

			labels := client.MatchingLabels{}
			labels["app.kubernetes.io/part-of"] = "strimzi-" + kafka.Parameters.ConnectCluster.String()
			err = kafka.Client.List(ctx, pods, labels)

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
