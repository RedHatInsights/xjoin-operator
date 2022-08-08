package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/go-errors/errors"
	"github.com/google/uuid"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers"
	xjoinconfig "github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"github.com/redhatinsights/xjoin-operator/test"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"net/http"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"text/template"
	"time"
)

var ResourceNamePrefix = "xjointest"

type Iteration struct {
	NamespacedName         types.NamespacedName
	XJoinReconciler        *controllers.XJoinPipelineReconciler
	ValidationReconciler   *controllers.ValidationReconciler
	KafkaConnectReconciler *controllers.KafkaConnectReconciler
	EsClient               *elasticsearch.ElasticSearch
	KafkaClient            kafka.Kafka
	KafkaConnectors        kafka.Connectors
	KafkaTopics            kafka.Topics
	Parameters             xjoinconfig.Parameters
	ParametersMap          map[string]interface{}
	DbClient               *database.Database
	Pipelines              []*xjoin.XJoinPipeline
	Now                    time.Time
}

func NewTestIteration() *Iteration {
	testIteration := Iteration{}
	now := time.Now().UTC()
	now = now.Add(-time.Duration(600) * time.Second)
	testIteration.Now = now
	return &testIteration
}

func (i *Iteration) CheckIfDependenciesAreResponding() bool {
	err := i.DbClient.Connect()
	if err != nil {
		return false
	}
	isResponding, err := i.KafkaConnectors.CheckIfConnectIsResponding()
	if err != nil || !isResponding {
		return false
	}
	_, err = i.EsClient.ListIndices()
	if err != nil {
		return false
	}

	return true
}

func (i *Iteration) CloseDB() error {
	if i.DbClient != nil {
		err := i.DbClient.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *Iteration) getConnectorConfig(connectorName string) (map[string]interface{}, error) {
	configUrl := fmt.Sprintf(
		"http://%s-connect-api.%s.svc:8083/connectors/%s/config",
		i.Parameters.ConnectCluster.String(), i.Parameters.ConnectClusterNamespace.String(), connectorName)

	//GET current config from Kafka Connect REST API
	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, configUrl, nil)
	if err != nil {
		return nil, err
	}
	res, err := httpClient.Do(req)

	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, errors.New(fmt.Sprintf("invalid response code during getConnectorConfig: %v", res.StatusCode))
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var bodyMap map[string]interface{}
	err = json.Unmarshal(body, &bodyMap)
	if err != nil {
		return nil, err
	}
	return bodyMap, nil
}

func (i *Iteration) putConnectorConfig(connectorName string, config []byte) error {
	configUrl := fmt.Sprintf(
		"http://%s-connect-api.%s.svc:8083/connectors/%s/config",
		i.Parameters.ConnectCluster.String(), i.Parameters.ConnectClusterNamespace.String(), connectorName)

	//PUT updated config
	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, configUrl, bytes.NewReader(config))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	if res == nil {
		return errors.New("empty response during putConnectorConfig")
	}
	err = res.Body.Close()
	return err
}

func (i *Iteration) SetDBConnectorHost(dbHost string, connectorName string) error {
	bodyMap, err := i.getConnectorConfig(connectorName)
	if err != nil {
		return err
	}

	//update config with new URL
	bodyMap["database.hostname"] = dbHost
	bodyJson, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}

	err = i.putConnectorConfig(connectorName, bodyJson)
	return err
}

func (i *Iteration) SetESConnectorURL(esUrl string, connectorName string) error {
	bodyMap, err := i.getConnectorConfig(connectorName)
	if err != nil {
		return err
	}

	//update config with new URL
	bodyMap["connection.url"] = esUrl
	bodyJson, err := json.Marshal(bodyMap)
	if err != nil {
		return err
	}

	err = i.putConnectorConfig(connectorName, bodyJson)
	return err
}

func (i *Iteration) DeleteService(serviceName string) error {
	service := &corev1.Service{}
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	err := i.XJoinReconciler.Client.Get(
		ctx, client.ObjectKey{Name: serviceName, Namespace: "test"}, service)
	if err != nil {
		return err
	}

	err = i.XJoinReconciler.Client.Delete(ctx, service)
	return err
}

func (i *Iteration) CreateDBService(serviceName string) error {
	existingService := &corev1.Service{}
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	err := i.XJoinReconciler.Client.Get(
		ctx, client.ObjectKey{Name: "host-inventory-db", Namespace: "test"}, existingService)
	if err != nil {
		return err
	}

	newService := &corev1.Service{}
	newService.Name = serviceName
	newService.Namespace = "test"
	newService.Spec.Selector = existingService.Spec.Selector
	newService.Spec.Ports = existingService.Spec.Ports

	err = i.XJoinReconciler.Client.Create(ctx, newService)
	return err
}

func (i *Iteration) CreateESService(serviceName string) error {
	service := &corev1.Service{}
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	err := i.XJoinReconciler.Client.Get(
		ctx, client.ObjectKey{Name: "xjoin-elasticsearch-es-default", Namespace: "test"}, service)
	if err != nil {
		return err
	}

	service.Name = serviceName
	service.Namespace = "test"
	service.Spec.ClusterIP = ""
	service.Spec.ClusterIPs = []string{}
	service.ResourceVersion = ""
	err = i.XJoinReconciler.Client.Create(ctx, service)
	return err
}

func (i *Iteration) ScaleDeployment(name string, namespace string, replicas int) error {
	var deploymentGVK = schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "Deployment",
		Version: "v1",
	}

	//get existing strimzi deployment
	deployment := &unstructured.Unstructured{}
	deployment.SetGroupVersionKind(deploymentGVK)
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err := i.XJoinReconciler.Client.Get(
		ctx, client.ObjectKey{Name: name, Namespace: namespace}, deployment)
	if err != nil {
		return err
	}

	//update deployment with replicas
	obj := deployment.Object
	spec := obj["spec"].(map[string]interface{})
	spec["replicas"] = replicas

	ctx, cancel = utils.DefaultContext()
	defer cancel()
	err = i.KafkaClient.Client.Update(ctx, deployment)
	if err != nil {
		return err
	}

	//wait for deployment to be ready if replicas > 0
	if replicas > 0 {
		err = wait.PollImmediate(time.Second, time.Duration(150)*time.Second, func() (bool, error) {
			pods := &corev1.PodList{}

			ctx, cancel = utils.DefaultContext()
			defer cancel()

			labels := client.MatchingLabels{}
			labels["name"] = name
			err = i.XJoinReconciler.Client.List(ctx, pods, labels)

			if len(pods.Items) == 0 {
				return false, nil
			}

			for _, condition := range pods.Items[0].Status.Conditions {
				if condition.Type == "Ready" {
					if condition.Status == "True" {
						return true, nil
					} else {
						return false, nil
					}
				}
			}

			return false, nil
		})

		if err != nil {
			return err
		}
	} else {
		err = wait.PollImmediate(time.Second, time.Duration(150)*time.Second, func() (bool, error) {
			pods := &corev1.PodList{}

			ctx, cancel = utils.DefaultContext()
			defer cancel()

			labels := client.MatchingLabels{}
			labels["name"] = name
			err = i.XJoinReconciler.Client.List(ctx, pods, labels)

			if len(pods.Items) > 0 {
				return false, nil
			} else {
				return true, nil
			}
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *Iteration) EditESConnectorToBeInvalid(pipelineVersion string) error {
	connector, err := i.KafkaClient.GetConnector(i.KafkaConnectors.ESConnectorName(pipelineVersion))
	if err != nil {
		return err
	}

	obj := connector.Object
	spec := obj["spec"].(map[string]interface{})
	config := spec["config"].(map[string]interface{})
	config["connection.url"] = "invalid"

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = i.KafkaClient.Client.Update(ctx, connector)
	return err
}

func (i *Iteration) TestSpecFieldChangedForPipeline(
	pipeline *xjoin.XJoinPipeline,
	fieldName string,
	fieldValue interface{},
	valueType reflect.Kind) error {

	activeIndex := pipeline.Status.ActiveIndexName

	s := reflect.ValueOf(&pipeline.Spec).Elem()
	field := s.FieldByName(fieldName)
	if valueType == reflect.String {
		val := fieldValue.(string)
		field.Set(reflect.ValueOf(&val))
	} else if valueType == reflect.Int {
		val := fieldValue.(int)
		field.Set(reflect.ValueOf(&val))
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err := test.Client.Update(ctx, pipeline)
	if err != nil {
		return err
	}
	pipeline, err = i.ExpectInitSyncUnknownReconcile()
	if err != nil {
		return err
	}
	if pipeline.Status.ActiveIndexName != activeIndex {
		return errors.New("pipeline.Status.ActiveIndexName != activeIndex")
	}

	return nil
}

func (i *Iteration) TestSpecFieldChanged(fieldName string, fieldValue interface{}, valueType reflect.Kind) error {
	pipeline, err := i.CreateValidPipeline()
	if err != nil {
		return err
	}
	return i.TestSpecFieldChangedForPipeline(pipeline, fieldName, fieldValue, valueType)
}

func (i *Iteration) CopySecret(existingSecretName string, newSecretName string, existingNamespace string, newNamespace string) error {
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	secret := &corev1.Secret{}
	err := test.Client.Get(ctx, client.ObjectKey{Name: existingSecretName, Namespace: existingNamespace}, secret)
	if err != nil {
		return err
	}

	newSecret := &corev1.Secret{}
	newSecret.Name = newSecretName
	newSecret.Data = secret.Data
	newSecret.Data["newkey"] = nil
	newSecret.Namespace = newNamespace

	return test.Client.Create(ctx, newSecret)
}

func (i *Iteration) DeleteAllHosts() error {
	rows, err := i.DbClient.RunQuery("DELETE FROM hosts;")
	if err != nil {
		return err
	}
	return rows.Close()
}

func (i *Iteration) SyncHosts(pipelineVersion string, numHosts int) ([]string, error) {
	var ids []string

	for j := 0; j < numHosts; j++ {
		id, err := i.InsertSimpleHost()
		if err != nil {
			return nil, err
		}
		err = i.IndexSimpleDocument(pipelineVersion, id)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}

func (i *Iteration) IndexSimpleDocument(pipelineVersion string, id string) error {
	return i.IndexDocument(pipelineVersion, id, "simple", i.Now)
}

func (i *Iteration) IndexDocumentNow(pipelineVersion string, id string, filename string) error {
	return i.IndexDocument(pipelineVersion, id, filename, i.Now)
}

func (i *Iteration) IndexDocument(pipelineVersion string, id string, filename string, modifiedOn time.Time) error {
	esDocumentFile, err := ioutil.ReadFile(test.GetRootDir() + "/test/hosts/es/" + filename + ".json")
	if err != nil {
		return err
	}

	tmpl, err := template.New("esDocumentTemplate").Parse(string(esDocumentFile))
	if err != nil {
		return err
	}

	m := make(map[string]interface{})
	m["ID"] = id
	m["ModifiedOn"] = modifiedOn.Format(time.RFC3339)

	var templateBuffer bytes.Buffer
	err = tmpl.Execute(&templateBuffer, m)
	if err != nil {
		return err
	}
	templateParsed := templateBuffer.String()

	// Set up the request object.
	req := esapi.IndexRequest{
		Index:      i.EsClient.ESIndexName(pipelineVersion),
		DocumentID: id,
		Body:       strings.NewReader(strings.ReplaceAll(templateParsed, "\n", "")),
		Refresh:    "true",
	}

	// Perform the request with the client.
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	res, err := req.Do(ctx, i.EsClient.Client)
	if err != nil {
		return err
	}

	if res.IsError() {

		bodyBytes, err := ioutil.ReadAll(res.Body)
		err = res.Body.Close()
		if err != nil {
			return err
		}

		errorMsg := fmt.Sprintf("elasticsearch API error: %s, %s", strconv.Itoa(res.StatusCode), bodyBytes)
		log.Error(errors.New(errorMsg), "error indexing document")
		if res.IsError() {
			return errors.New(errorMsg)
		}
	}

	return nil
}

func (i *Iteration) InsertSimpleHost() (string, error) {
	return i.InsertHost("simple", i.Now)
}

func (i *Iteration) InsertHostNow(filename string) (string, error) {
	return i.InsertHost(filename, i.Now)
}

func (i *Iteration) InsertHost(filename string, modifiedOn time.Time) (string, error) {
	hostId, err := uuid.NewUUID()

	hbiHostFile, err := ioutil.ReadFile(test.GetRootDir() + "/test/hosts/hbi/" + filename + ".sql")
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("hbiHostTemplate").Parse(string(hbiHostFile))
	if err != nil {
		return "", err
	}

	m := make(map[string]interface{})
	m["ID"] = hostId.String()
	m["ModifiedOn"] = modifiedOn.Format(time.RFC3339)

	var templateBuffer bytes.Buffer
	err = tmpl.Execute(&templateBuffer, m)
	if err != nil {
		return "", err
	}
	query := strings.ReplaceAll(templateBuffer.String(), "\n", "")

	_, err = i.DbClient.ExecQuery(query)
	if err != nil {
		return "", err
	}

	return hostId.String(), nil
}

func (i *Iteration) CreateConfigMap(name string, data map[string]string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.NamespacedName.Namespace,
		},
		Data: data,
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	return test.Client.Create(ctx, configMap)
}

func (i *Iteration) CreateESSecret(name string) error {
	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.NamespacedName.Namespace,
		},
		Data: map[string][]byte{
			"endpoint": []byte(i.Parameters.ElasticSearchURL.String()),
			"username": []byte(i.Parameters.ElasticSearchUsername.String()),
			"password": []byte(i.Parameters.ElasticSearchPassword.String()),
		},
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	return test.Client.Create(ctx, secret)
}

func (i *Iteration) CreateDbSecret(name string) error {
	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.NamespacedName.Namespace,
		},
		Data: map[string][]byte{
			"db.host":     []byte(i.Parameters.HBIDBHost.String()),
			"db.port":     []byte(i.Parameters.HBIDBPort.String()),
			"db.name":     []byte(i.Parameters.HBIDBName.String()),
			"db.user":     []byte(i.Parameters.HBIDBUser.String()),
			"db.password": []byte(i.Parameters.HBIDBPassword.String()),
		},
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	return test.Client.Create(ctx, secret)
}

func (i *Iteration) ExpectPipelineVersionToBeRemoved(pipelineVersion string) error {
	exists, err := i.EsClient.IndexExists(i.EsClient.ESIndexName(pipelineVersion))
	if err != nil {
		return err
	}
	if exists != false {
		return errors.New(fmt.Sprintf("expected index %s to not exist", i.EsClient.ESIndexName(pipelineVersion)))
	}

	slots, err := i.DbClient.ListReplicationSlots(ResourceNamePrefix)
	if err != nil {
		return err
	}
	if len(slots) != 0 {
		return errors.New("expected all replication slots to be removed for prefix: " + ResourceNamePrefix)
	}

	versions, err := i.KafkaTopics.ListTopicNamesForPipelineVersion(pipelineVersion)
	if err != nil {
		return err
	}
	if len(versions) != 0 {
		return errors.New("expected no topic names to exist for version: " + pipelineVersion)
	}

	connectors, err := i.KafkaConnectors.ListConnectorNamesForPipelineVersion(pipelineVersion)
	if err != nil {
		return err
	}
	if len(connectors) != 0 {
		return errors.New("expected no connectors for version: " + pipelineVersion)
	}

	esPipelines, err := i.EsClient.ListESPipelines(pipelineVersion)
	if err != nil {
		return err
	}
	if len(esPipelines) != 0 {
		return errors.New("expected no pipelines to exist for version: " + pipelineVersion)
	}

	return nil
}

func (i *Iteration) ExpectValidReconcile() (*xjoin.XJoinPipeline, error) {
	_, err := i.ReconcileValidation()
	if err != nil {
		return nil, err
	}
	pipeline, err := i.ReconcileXJoin()
	if err != nil {
		return nil, err
	}

	if pipeline.GetState() != xjoin.STATE_VALID {
		return nil, errors.New("expected pipeline state to be valid")
	}

	if pipeline.Status.InitialSyncInProgress != false {
		return nil, errors.New("expected InitialSyncInProgress to be false")
	}

	if pipeline.GetValid() != metav1.ConditionTrue {
		return nil, errors.New("expected pipeline to be valid")
	}

	return pipeline, nil
}

func (i *Iteration) ExpectNewReconcile() (*xjoin.XJoinPipeline, error) {
	_, err := i.ReconcileValidation()
	if err != nil {
		return nil, err
	}
	pipeline, err := i.ReconcileXJoin()
	if err != nil {
		return nil, err
	}

	if pipeline.GetState() != xjoin.STATE_NEW {
		return nil, errors.New("expected pipeline state to be new")
	}

	if pipeline.Status.InitialSyncInProgress != false {
		return nil, errors.New("expected InitialSyncInProgress to be false")
	}

	if pipeline.GetValid() != metav1.ConditionUnknown {
		return nil, errors.New("expected pipeline condition to be unknown")
	}

	return pipeline, nil
}

func (i *Iteration) ExpectInitSyncInvalidReconcile() (*xjoin.XJoinPipeline, error) {
	_, err := i.ReconcileValidation()
	if err != nil {
		return nil, err
	}
	pipeline, err := i.ReconcileXJoin()
	if err != nil {
		return nil, err
	}

	if pipeline.GetState() != xjoin.STATE_INITIAL_SYNC {
		return nil, errors.New("expected pipeline state to be initial_sync")
	}

	if pipeline.Status.InitialSyncInProgress != true {
		return nil, errors.New("expected InitialSyncInProgress to be true")
	}

	if pipeline.GetValid() != metav1.ConditionFalse {
		return nil, errors.New("expected pipeline to not be valid")
	}
	return pipeline, nil
}

func (i *Iteration) ExpectInitSyncUnknownReconcile() (*xjoin.XJoinPipeline, error) {
	_, err := i.ReconcileValidation()
	if err != nil {
		return nil, err
	}
	pipeline, err := i.ReconcileXJoin()
	if err != nil {
		return nil, err
	}

	if pipeline.GetState() != xjoin.STATE_INITIAL_SYNC {
		return nil, errors.New("expected pipeline state to be initial_sync")
	}

	if pipeline.Status.InitialSyncInProgress != true {
		return nil, errors.New("expected InitialSyncInProgress to be true")
	}

	if pipeline.GetValid() != metav1.ConditionUnknown {
		return nil, errors.New("expected pipeline condition to be unknown")
	}
	return pipeline, nil
}

func (i *Iteration) ExpectInvalidReconcile() (*xjoin.XJoinPipeline, error) {
	_, err := i.ReconcileValidation()
	if err != nil {
		return nil, err
	}
	pipeline, err := i.ReconcileXJoin()
	if err != nil {
		return nil, err
	}

	if pipeline.GetState() != xjoin.STATE_INVALID {
		return nil, errors.New("expected pipeline state to be invalid")
	}

	if pipeline.Status.InitialSyncInProgress != false {
		return nil, errors.New("expected InitialSyncInProgress to be false")
	}

	if pipeline.GetValid() != metav1.ConditionFalse {
		return nil, errors.New("expected pipeline to be invalid")
	}
	return pipeline, nil
}

func (i *Iteration) CreateValidPipeline(specs ...*xjoin.XJoinPipelineSpec) (*xjoin.XJoinPipeline, error) {
	err := i.CreatePipeline(specs...)
	if err != nil {
		return nil, err
	}
	_, err = i.ReconcileXJoin()
	if err != nil {
		return nil, err
	}
	_, err = i.ReconcileValidation()
	if err != nil {
		return nil, err
	}

	pipeline, err := i.ReconcileXJoin()
	if err != nil {
		return nil, err
	}

	if pipeline.GetState() != xjoin.STATE_VALID {
		return nil, errors.New("expected pipeline state to be valid")
	}

	if pipeline.Status.InitialSyncInProgress != false {
		return nil, errors.New("expected InitialSyncInProgress to be false")
	}

	if pipeline.GetValid() != metav1.ConditionTrue {
		return nil, errors.New("expected pipeline to be invalid")
	}

	return pipeline, nil
}

func (i *Iteration) DeletePipeline(pipeline *xjoin.XJoinPipeline) error {
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err := test.Client.Delete(ctx, pipeline)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	err = i.ReconcileValidationForDeletedPipeline()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	err = i.ReconcileXJoinForDeletedPipeline()
	return errors.Wrap(err, 0)
}

func (i *Iteration) CreatePipeline(specs ...*xjoin.XJoinPipelineSpec) error {
	var spec *xjoin.XJoinPipelineSpec

	if len(specs) > 1 {
		return errors.New("only one spec is allowed")
	}

	namespace := "test"
	connectCluster := "connect"
	kafkaCluster := "kafka"
	if len(specs) == 1 {
		spec = specs[0]
		if specs[0].ResourceNamePrefix == nil {
			specs[0].ResourceNamePrefix = &ResourceNamePrefix
		}

		if specs[0].ConnectCluster == nil {
			specs[0].ConnectCluster = &connectCluster
		}

		if specs[0].ConnectClusterNamespace == nil {
			specs[0].ConnectClusterNamespace = &namespace
		}

		if specs[0].KafkaCluster == nil {
			specs[0].KafkaCluster = &kafkaCluster
		}

		if specs[0].KafkaClusterNamespace == nil {
			specs[0].KafkaClusterNamespace = &namespace
		}

		if specs[0].ElasticSearchNamespace == nil {
			specs[0].ElasticSearchNamespace = &namespace
		}
	} else {
		spec = &xjoin.XJoinPipelineSpec{
			ResourceNamePrefix:      &ResourceNamePrefix,
			ConnectClusterNamespace: &namespace,
			KafkaClusterNamespace:   &namespace,
			ConnectCluster:          &connectCluster,
			KafkaCluster:            &kafkaCluster,
			ElasticSearchNamespace:  &namespace,
		}
	}

	pipeline := xjoin.XJoinPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.NamespacedName.Name,
			Namespace: i.NamespacedName.Namespace,
		},
		Spec: *spec,
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err := test.Client.Create(ctx, &pipeline)
	if err != nil {
		return err
	}
	i.Pipelines = append(i.Pipelines, &pipeline)
	return nil
}

func (i *Iteration) GetPipeline() (*xjoin.XJoinPipeline, error) {
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	return utils.FetchXJoinPipeline(test.Client, i.NamespacedName, ctx)
}

func (i *Iteration) ReconcileXJoinForDeletedPipeline() error {
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	result, err := i.XJoinReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: i.NamespacedName})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if result.Requeue != false {
		return errors.Wrap(errors.New("expected result.Requeue to be false"), 0)
	}

	return nil
}

func (i *Iteration) ReconcileValidationWithError() error {
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	_, err := i.ValidationReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: i.NamespacedName})
	return err
}

func (i *Iteration) ReconcileXJoinWithError() error {
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	_, err := i.XJoinReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: i.NamespacedName})
	return err
}

func (i *Iteration) ReconcileXJoinNonTestWithError() error {
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	xJoinReconciler := newXJoinReconciler(i.NamespacedName.Namespace, false)
	_, err := xJoinReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: i.NamespacedName})
	return err
}

func (i *Iteration) ReconcileXJoinNonTest() (*xjoin.XJoinPipeline, error) {
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	xJoinReconciler := newXJoinReconciler(i.NamespacedName.Namespace, false)
	result, err := xJoinReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: i.NamespacedName})
	if err != nil {
		return nil, err
	}

	if result.Requeue != false {
		return nil, errors.New("expected result.requeue to be false")
	}
	return i.GetPipeline()
}

func (i *Iteration) ReconcileKafkaConnect() (bool, error) {
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	result, err := i.KafkaConnectReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: i.NamespacedName})
	if err != nil {
		return false, err
	}
	return result.Requeue, err
}

func (i *Iteration) ReconcileXJoin() (*xjoin.XJoinPipeline, error) {
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	result, err := i.XJoinReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: i.NamespacedName})
	if err != nil {
		return nil, err
	}
	if result.Requeue != false {
		return nil, errors.New("expected result.Requeue to be false")
	}
	return i.GetPipeline()
}

func (i *Iteration) ReconcileValidationForDeletedPipeline() error {
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	result, err := i.ValidationReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: i.NamespacedName})
	if err != nil {
		return err
	}
	if result.Requeue != false {
		return errors.New("expected result.Requeue to be false")
	}

	return nil
}

func (i *Iteration) ReconcileValidation() (*xjoin.XJoinPipeline, error) {
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	result, err := i.ValidationReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: i.NamespacedName})
	if err != nil {
		return nil, err
	}

	if result.Requeue != false {
		return nil, errors.New("expected result.requeue to be false")
	}
	return i.GetPipeline()
}

func (i *Iteration) AssertValidationEvents(numHosts int) error {
	recorder, _ := i.ValidationReconciler.Recorder.(*record.FakeRecorder)
	if len(recorder.Events) != 3 {
		return errors.New("expected 3 events")
	}
	msg := <-recorder.Events
	expectedMsg := fmt.Sprintf("Normal CountValidationPassed Results: mismatchRatio: 0, esCount: %v, hbiCount: %v", numHosts, numHosts)
	if msg != expectedMsg {
		return errors.New(fmt.Sprintf("expected msg %s to equal %s", msg, expectedMsg))
	}
	msg = <-recorder.Events
	expectedMsg = fmt.Sprintf("Normal IDValidationPassed 0 hosts ids do not match. Number of hosts IDs retrieved: HBI: %v, ES: %v", numHosts, numHosts)
	if msg != expectedMsg {
		return errors.New(fmt.Sprintf("expected msg %s to equal %s", msg, expectedMsg))
	}

	msg = <-recorder.Events
	expectedMsg = fmt.Sprintf("Normal FullValidationPassed 0 hosts do not match. %v hosts validated.", numHosts)
	if msg != expectedMsg {
		return errors.New(fmt.Sprintf("expected msg %s to equal %s", msg, expectedMsg))
	}

	return nil
}

func (i *Iteration) WaitForPipelineToBeValid() (*xjoin.XJoinPipeline, error) {
	var pipeline *xjoin.XJoinPipeline

	for j := 0; j < 10; j++ {
		_, err := i.ReconcileValidation()
		if err != nil {
			return nil, err
		}
		pipeline, err = i.GetPipeline()
		if err != nil {
			return nil, err
		}

		if pipeline.GetState() == xjoin.STATE_VALID {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return pipeline, nil
}

func (i *Iteration) setPrefix(prefix string) error {
	err := i.KafkaClient.Parameters.ResourceNamePrefix.SetValue(prefix)
	if err != nil {
		return err
	}
	i.EsClient.SetResourceNamePrefix(prefix)
	return i.Parameters.ResourceNamePrefix.SetValue(prefix)
}

func (i *Iteration) fullValidationFailureTest(hbiFileName string, esFileName string) error {
	pipeline, err := i.CreateValidPipeline()
	if err != nil {
		return err
	}
	err = i.AssertValidationEvents(0)
	if err != nil {
		return err
	}

	_, err = i.SyncHosts(pipeline.Status.PipelineVersion, 3)
	if err != nil {
		return err
	}

	hostId, err := i.InsertHostNow(hbiFileName)
	if err != nil {
		return err
	}
	err = i.IndexDocumentNow(pipeline.Status.PipelineVersion, hostId, esFileName)
	if err != nil {
		return err
	}

	pipeline, err = i.ReconcileValidation()
	if err != nil {
		return err
	}
	if pipeline.GetState() != xjoin.STATE_INVALID {
		return errors.New("expected pipeline state to be invalid")
	}
	if pipeline.GetValid() != metav1.ConditionFalse {
		return errors.New("expected pipeline to be invalid")
	}

	recorder, _ := i.ValidationReconciler.Recorder.(*record.FakeRecorder)
	if len(recorder.Events) != 3 {
		return errors.New("expected recorder.Events to have length of 3")
	}

	msg := <-recorder.Events
	expectedMsg := "Normal CountValidationPassed Results: mismatchRatio: 0, esCount: 4, hbiCount: 4"
	if msg != expectedMsg {
		return errors.New(fmt.Sprintf("invalid event message: %s, expected: %s", msg, expectedMsg))
	}

	msg = <-recorder.Events
	expectedMsg = "Normal IDValidationPassed 0 hosts ids do not match. Number of hosts IDs retrieved: HBI: 4, ES: 4"
	if msg != expectedMsg {
		return errors.New(fmt.Sprintf("invalid event message: %s, expected: %s", msg, expectedMsg))
	}

	msg = <-recorder.Events
	expectedMsg = "Normal FullValidationFailed 1 hosts do not match. 4 hosts validated."
	if msg != expectedMsg {
		return errors.New(fmt.Sprintf("invalid event message: %s, expected: %s", msg, expectedMsg))
	}

	return nil
}

func (i *Iteration) getConnectPodName() (string, error) {
	pods := &corev1.PodList{}

	ctx, cancel := utils.DefaultContext()
	defer cancel()

	labels := client.MatchingLabels{}
	labels["app.kubernetes.io/part-of"] = "strimzi-connect"
	err := i.KafkaConnectReconciler.Client.List(ctx, pods, labels)
	if err != nil {
		return "", err
	}

	if len(pods.Items) == 0 {
		return "", errors.New("no kafka connect pods found")
	}

	for _, pod := range pods.Items {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				return pod.Name, nil
			}
		}
	}

	return "", errors.New("unable to get pod name, it might not be ready yet")
}

func (i *Iteration) WaitForConnectorToBeCreated(connectorName string) error {
	attempts := 30

	for j := 0; j < attempts; j++ {
		exists, err := i.KafkaClient.CheckConnectorExistsViaREST(connectorName)
		if err != nil {
			return err
		}

		if exists {
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return errors.New("timed out waiting for connector to be created")
}

func (i *Iteration) WaitForConnectorTaskToFail(connectorName string) error {
	isFailed := false
	for j := 0; j < 60; j++ {
		tasks, err := i.KafkaConnectors.ListConnectorTasks(connectorName)
		if err != nil {
			return err
		}
		for _, task := range tasks {
			if task["state"] == "FAILED" {
				isFailed = true
				break
			}
		}

		time.Sleep(1 * time.Second)
	}

	if !isFailed {
		return errors.New(fmt.Sprintf("timed out waiting for connector %s task to fail", connectorName))
	}

	return nil
}

func (i *Iteration) PauseConnectorReconciliation(connectorName string) error {
	return i.setConnectorReconciliationPause(connectorName, "true")
}

func (i *Iteration) ResumeConnectorReconciliation(connectorName string) (err error) {
	return i.setConnectorReconciliationPause(connectorName, "false")
}

func (i *Iteration) setConnectorReconciliationPause(connectorName string, pause string) (err error) {
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	//strimzi might update the connector between the GET/PUT calls, so retry a few times
	for j := 0; j < 5; j++ {
		connector, err := i.KafkaClient.GetConnector(connectorName)
		if err != nil {
			return err
		}
		annotations := make(map[string]interface{})
		annotations["strimzi.io/pause-reconciliation"] = pause
		metadata := connector.Object["metadata"].(map[string]interface{})
		metadata["annotations"] = annotations
		err = test.Client.Update(ctx, connector)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	return err
}
