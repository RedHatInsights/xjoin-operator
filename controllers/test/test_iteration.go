package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
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
	"net/http"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"text/template"
	"time"
)

var ResourceNamePrefix = "xjointest"

type Iteration struct {
	NamespacedName       types.NamespacedName
	XJoinReconciler      *controllers.XJoinPipelineReconciler
	ValidationReconciler *controllers.ValidationReconciler
	EsClient             *elasticsearch.ElasticSearch
	KafkaClient          kafka.Kafka
	Parameters           xjoinconfig.Parameters
	ParametersMap        map[string]interface{}
	DbClient             *database.Database
	Pipelines            []*xjoin.XJoinPipeline
}

func NewTestIteration() *Iteration {
	testIteration := Iteration{}
	return &testIteration
}

func (i *Iteration) SetESConnectorURL(esUrl string, connectorName string) {
	configUrl := fmt.Sprintf(
		"http://%s-connect-api.%s.svc:8083/connectors/%s/config",
		i.Parameters.ConnectCluster.String(), i.Parameters.ConnectClusterNamespace.String(), connectorName)

	res, err := http.Get(configUrl)
	Expect(err).ToNot(HaveOccurred())
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	var bodyMap map[string]interface{}
	err = json.Unmarshal(body, &bodyMap)
	Expect(err).ToNot(HaveOccurred())

	bodyMap["connection.url"] = esUrl
	bodyJson, err := json.Marshal(bodyMap)
	Expect(err).ToNot(HaveOccurred())

	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodPut, configUrl, bytes.NewReader(bodyJson))
	Expect(err).ToNot(HaveOccurred())
	req.Header.Add("Content-Type", "application/json")
	res, err = httpClient.Do(req)
	Expect(err).ToNot(HaveOccurred())
	Expect(res).ToNot(BeNil())
}

func (i *Iteration) DeleteService(serviceName string) {
	service := &corev1.Service{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	err := i.XJoinReconciler.Client.Get(
		ctx, client.ObjectKey{Name: serviceName, Namespace: "xjoin-operator-project"}, service)
	Expect(err).ToNot(HaveOccurred())

	err = i.XJoinReconciler.Client.Delete(ctx, service)
	Expect(err).ToNot(HaveOccurred())
}

func (i *Iteration) CreateService(serviceName string) {
	service := &corev1.Service{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	err := i.XJoinReconciler.Client.Get(
		ctx, client.ObjectKey{Name: "xjoin-elasticsearch-es-default", Namespace: "xjoin-operator-project"}, service)
	Expect(err).ToNot(HaveOccurred())

	service.Name = serviceName
	service.Namespace = "xjoin-operator-project"
	service.Spec.ClusterIP = ""
	service.Spec.ClusterIPs = []string{}
	service.ResourceVersion = ""
	err = i.XJoinReconciler.Client.Create(ctx, service)
	Expect(err).ToNot(HaveOccurred())
}

func (i *Iteration) ScaleStrimziDeployment(replicas int) {
	var deploymentGVK = schema.GroupVersionKind{
		Group:   "apps",
		Kind:    "Deployment",
		Version: "v1",
	}

	//get existing strimzi deployment
	deployment := &unstructured.Unstructured{}
	deployment.SetGroupVersionKind(deploymentGVK)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := i.XJoinReconciler.Client.Get(
		ctx, client.ObjectKey{Name: "strimzi-cluster-operator", Namespace: "kafka"}, deployment)

	Expect(err).ToNot(HaveOccurred())

	//update deployment with replicas
	obj := deployment.Object
	spec := obj["spec"].(map[string]interface{})
	spec["replicas"] = replicas

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err = i.KafkaClient.Client.Update(ctx, deployment)
	Expect(err).ToNot(HaveOccurred())

	//wait for deployment to be ready if replicas > 0
	if replicas > 0 {
		err = wait.PollImmediate(time.Second, time.Duration(60)*time.Second, func() (bool, error) {
			pods := &corev1.PodList{}

			ctx, cancel = context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()

			labels := client.MatchingLabels{}
			labels["name"] = "strimzi-cluster-operator"
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
		Expect(err).ToNot(HaveOccurred())
	}
}

func (i *Iteration) EditESConnectorToBeInvalid(pipelineVersion string) {
	connector, err := i.KafkaClient.GetConnector(i.KafkaClient.ESConnectorName(pipelineVersion))
	Expect(err).ToNot(HaveOccurred())

	obj := connector.Object
	spec := obj["spec"].(map[string]interface{})
	config := spec["config"].(map[string]interface{})
	config["connection.url"] = "invalid"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err = i.KafkaClient.Client.Update(ctx, connector)
	Expect(err).ToNot(HaveOccurred())
}

func (i *Iteration) TestSpecFieldChanged(fieldName string, fieldValue interface{}, valueType reflect.Kind) {
	pipeline := i.CreateValidPipeline()
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := test.Client.Update(ctx, pipeline)
	Expect(err).ToNot(HaveOccurred())
	pipeline = i.ExpectInitSyncUnknownReconcile()
	Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))
}

func (i *Iteration) DeleteAllHosts() {
	rows, err := i.DbClient.RunQuery("DELETE FROM hosts;")
	Expect(err).ToNot(HaveOccurred())
	rows.Close()
}

func (i *Iteration) SyncHosts(pipelineVersion string, numHosts int) []string {
	var ids []string

	for j := 0; j < numHosts; j++ {
		id := i.InsertHost()
		i.IndexDocument(pipelineVersion, id)
		ids = append(ids, id)
	}

	return ids
}

func (i *Iteration) IndexDocument(pipelineVersion string, id string) {
	esDocumentFile, err := ioutil.ReadFile("./test/es.document.json")
	Expect(err).ToNot(HaveOccurred())

	tmpl, err := template.New("esDocumentTemplate").Parse(string(esDocumentFile))
	Expect(err).ToNot(HaveOccurred())

	m := make(map[string]interface{})
	m["ID"] = id

	var templateBuffer bytes.Buffer
	err = tmpl.Execute(&templateBuffer, m)
	Expect(err).ToNot(HaveOccurred())
	templateParsed := templateBuffer.String()

	// Set up the request object.
	req := esapi.IndexRequest{
		Index:      i.EsClient.ESIndexName(pipelineVersion),
		DocumentID: id,
		Body:       strings.NewReader(strings.ReplaceAll(templateParsed, "\n", "")),
		Refresh:    "true",
	}

	// Perform the request with the client.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	res, err := req.Do(ctx, i.EsClient.Client)
	Expect(err).ToNot(HaveOccurred())

	err = res.Body.Close()
	Expect(err).ToNot(HaveOccurred())
}

func (i *Iteration) InsertHost() string {
	hostId, err := uuid.NewUUID()

	hbiHostFile, err := ioutil.ReadFile("./test/hbi.host.sql")
	Expect(err).ToNot(HaveOccurred())

	tmpl, err := template.New("hbiHostTemplate").Parse(string(hbiHostFile))
	Expect(err).ToNot(HaveOccurred())

	m := make(map[string]interface{})
	m["ID"] = hostId.String()

	var templateBuffer bytes.Buffer
	err = tmpl.Execute(&templateBuffer, m)
	Expect(err).ToNot(HaveOccurred())
	query := strings.ReplaceAll(templateBuffer.String(), "\n", "")

	rows, err := i.DbClient.RunQuery(query)
	Expect(err).ToNot(HaveOccurred())
	rows.Close()

	return hostId.String()
}

func (i *Iteration) CreateConfigMap(name string, data map[string]string) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.NamespacedName.Namespace,
		},
		Data: data,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := test.Client.Create(ctx, configMap)
	Expect(err).ToNot(HaveOccurred())
}

func (i *Iteration) CreateESSecret(name string) {
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := test.Client.Create(ctx, secret)
	Expect(err).ToNot(HaveOccurred())
}

func (i *Iteration) CreateDbSecret(name string) {
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := test.Client.Create(ctx, secret)
	Expect(err).ToNot(HaveOccurred())
}

func (i *Iteration) ExpectPipelineVersionToBeRemoved(pipelineVersion string) {
	exists, err := i.EsClient.IndexExists(i.EsClient.ESIndexName(pipelineVersion))
	Expect(err).ToNot(HaveOccurred())
	Expect(exists).To(Equal(false))

	slots, err := i.DbClient.ListReplicationSlots(ResourceNamePrefix)
	Expect(err).ToNot(HaveOccurred())
	Expect(len(slots)).To(Equal(0))

	versions, err := i.KafkaClient.ListTopicNamesForPipelineVersion(pipelineVersion)
	Expect(err).ToNot(HaveOccurred())
	Expect(len(versions)).To(Equal(0))

	connectors, err := i.KafkaClient.ListConnectorNamesForPipelineVersion(pipelineVersion)
	Expect(err).ToNot(HaveOccurred())
	Expect(len(connectors)).To(Equal(0))

	esPipelines, err := i.EsClient.ListESPipelines(pipelineVersion)
	Expect(err).ToNot(HaveOccurred())
	Expect(len(esPipelines)).To(Equal(0))
}

func (i *Iteration) ExpectValidReconcile() *xjoin.XJoinPipeline {
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))
	return pipeline
}

func (i *Iteration) ExpectNewReconcile() *xjoin.XJoinPipeline {
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_NEW))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))
	return pipeline
}

func (i *Iteration) ExpectInitSyncInvalidReconcile() *xjoin.XJoinPipeline {
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))
	return pipeline
}

func (i *Iteration) ExpectInitSyncUnknownReconcile() *xjoin.XJoinPipeline {
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))
	return pipeline
}

func (i *Iteration) ExpectInvalidReconcile() *xjoin.XJoinPipeline {
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INVALID))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))
	return pipeline
}

func (i *Iteration) CreateValidPipeline(specs ...*xjoin.XJoinPipelineSpec) *xjoin.XJoinPipeline {
	i.CreatePipeline(specs...)
	i.ReconcileXJoin()
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))

	return pipeline
}

func (i *Iteration) DeletePipeline(pipeline *xjoin.XJoinPipeline) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := test.Client.Delete(ctx, pipeline)
	Expect(err).ToNot(HaveOccurred())
	i.ReconcileValidationForDeletedPipeline()
	i.ReconcileXJoinForDeletedPipeline()
}

func (i *Iteration) CreatePipeline(specs ...*xjoin.XJoinPipelineSpec) {
	var spec *xjoin.XJoinPipelineSpec

	Expect(len(specs) <= 1).To(BeTrue())

	if len(specs) == 1 {
		spec = specs[0]
		if specs[0].ResourceNamePrefix == nil {
			specs[0].ResourceNamePrefix = &ResourceNamePrefix
		}
	} else {
		spec = &xjoin.XJoinPipelineSpec{
			ResourceNamePrefix: &ResourceNamePrefix,
		}
	}

	pipeline := xjoin.XJoinPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.NamespacedName.Name,
			Namespace: i.NamespacedName.Namespace,
		},
		Spec: *spec,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := test.Client.Create(ctx, &pipeline)
	Expect(err).ToNot(HaveOccurred())
	i.Pipelines = append(i.Pipelines, &pipeline)
}

func (i *Iteration) GetPipeline() (pipeline *xjoin.XJoinPipeline) {
	pipeline, err := utils.FetchXJoinPipeline(test.Client, i.NamespacedName)
	Expect(err).ToNot(HaveOccurred())
	return
}

func (i *Iteration) ReconcileXJoinForDeletedPipeline() {
	result, err := i.XJoinReconciler.Reconcile(ctrl.Request{NamespacedName: i.NamespacedName})
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())
}

func (i *Iteration) ReconcileValidationWithError() error {
	_, err := i.ValidationReconciler.Reconcile(ctrl.Request{NamespacedName: i.NamespacedName})
	Expect(err).To(HaveOccurred())
	return err
}

func (i *Iteration) ReconcileXJoinWithError() error {
	_, err := i.XJoinReconciler.Reconcile(ctrl.Request{NamespacedName: i.NamespacedName})
	Expect(err).To(HaveOccurred())
	return err
}

func (i *Iteration) ReconcileXJoin() *xjoin.XJoinPipeline {
	result, err := i.XJoinReconciler.Reconcile(ctrl.Request{NamespacedName: i.NamespacedName})
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())
	return i.GetPipeline()
}

func (i *Iteration) ReconcileValidationForDeletedPipeline() {
	result, err := i.ValidationReconciler.Reconcile(ctrl.Request{NamespacedName: i.NamespacedName})
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())
}

func (i *Iteration) ReconcileValidation() *xjoin.XJoinPipeline {
	result, err := i.ValidationReconciler.Reconcile(ctrl.Request{NamespacedName: i.NamespacedName})
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())
	return i.GetPipeline()
}

func (i *Iteration) WaitForPipelineToBeValid() *xjoin.XJoinPipeline {
	var pipeline *xjoin.XJoinPipeline

	for j := 0; j < 10; j++ {
		i.ReconcileValidation()
		pipeline = i.GetPipeline()

		if pipeline.GetState() == xjoin.STATE_VALID {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return pipeline
}

func (i *Iteration) setPrefix(prefix string) {
	err := i.KafkaClient.Parameters.ResourceNamePrefix.SetValue(prefix)
	Expect(err).ToNot(HaveOccurred())
	i.EsClient.SetResourceNamePrefix(prefix)
	err = i.Parameters.ResourceNamePrefix.SetValue(prefix)
	Expect(err).ToNot(HaveOccurred())
}

func (i *Iteration) cleanupJenkinsResources() {
	i.setPrefix("xjoin.inventory.hosts")

	_ = i.KafkaClient.DeleteConnector("xjoin.inventory.hosts.db.v1.1")
	_ = i.KafkaClient.DeleteConnector("xjoin.inventory.hosts.es.v1.1")
	_ = i.EsClient.DeleteIndex("v1.1")
	_ = i.EsClient.DeleteESPipelineByFullName("xjoin.inventory.hosts.v1.1")
	_ = i.KafkaClient.DeleteTopic("xjoin.inventory.v1.1.public.hosts")
	_ = i.DbClient.RemoveReplicationSlot("xjoin_inventory_v1_1")
}

func (i *Iteration) createJenkinsResources() {
	i.setPrefix("xjoin.inventory.hosts")

	//create resources to represent existing jenkins pipeline
	_, err := i.KafkaClient.CreateDebeziumConnector("v1.1", false)
	Expect(err).ToNot(HaveOccurred())

	_, err = i.KafkaClient.CreateESConnector("v1.1", false)
	Expect(err).ToNot(HaveOccurred())

	err = i.EsClient.CreateIndex("v1.1")
	Expect(err).ToNot(HaveOccurred())

	err = i.EsClient.CreateESPipeline("v1.1")
	Expect(err).ToNot(HaveOccurred())

	_, err = i.KafkaClient.CreateTopicByFullName("xjoin.inventory.v1.1.public.hosts", false)
	Expect(err).ToNot(HaveOccurred())

	err = i.DbClient.CreateReplicationSlot("xjoin_inventory_v1_1")
	Expect(err).ToNot(HaveOccurred())

	err = i.EsClient.UpdateAliasByFullIndexName("xjoin.inventory.hosts", "xjoin.inventory.hosts.v1.1")
	Expect(err).ToNot(HaveOccurred())
}

func (i *Iteration) validateJenkinsResourcesStillExist() {
	i.setPrefix("xjoin.inventory.hosts")

	dbConnector, err := i.KafkaClient.GetConnector("xjoin.inventory.hosts.db.v1.1")
	Expect(err).ToNot(HaveOccurred())
	Expect(dbConnector).ToNot(BeNil())
	Expect(dbConnector.GetName()).ToNot(BeEmpty())

	esConnector, err := i.KafkaClient.GetConnector("xjoin.inventory.hosts.es.v1.1")
	Expect(err).ToNot(HaveOccurred())
	Expect(esConnector).ToNot(BeNil())
	Expect(esConnector.GetName()).ToNot(BeEmpty())

	indices, err := i.EsClient.ListIndices()
	Expect(err).ToNot(HaveOccurred())
	Expect(indices).To(ContainElement("xjoin.inventory.hosts.v1.1"))

	pipelines, err := i.EsClient.ListESPipelines()
	Expect(err).ToNot(HaveOccurred())
	Expect(pipelines).To(ContainElement("xjoin.inventory.hosts.v1.1"))

	topic, err := i.KafkaClient.GetTopic("xjoin.inventory.v1.1.public.hosts")
	Expect(err).ToNot(HaveOccurred())
	Expect(topic).ToNot(BeNil())
	Expect(topic.GetName()).ToNot(BeEmpty())

	slots, err := i.DbClient.ListReplicationSlots("xjoin_inventory")
	Expect(err).ToNot(HaveOccurred())
	Expect(slots).To(ContainElements("xjoin_inventory_v1_1"))
}
