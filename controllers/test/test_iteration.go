package test

import (
	"bytes"
	"context"
	"github.com/elastic/go-elasticsearch/v7/esapi"
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
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
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

func (i *Iteration) EditESConnectorToBeInvalid(pipelineVersion string) {
	connector, err := i.KafkaClient.GetConnector(i.KafkaClient.ESConnectorName(pipelineVersion))
	Expect(err).ToNot(HaveOccurred())

	obj := connector.Object
	spec := obj["spec"].(map[string]interface{})
	config := spec["config"].(map[string]interface{})
	config["connection.url"] = "invalid"

	err = i.KafkaClient.Client.Update(context.Background(), connector)
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

	err := test.Client.Update(context.TODO(), pipeline)
	Expect(err).ToNot(HaveOccurred())
	pipeline = i.ExpectInitSyncReconcile()
	Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))
}

func (i *Iteration) DeleteAllHosts() {
	rows, err := i.DbClient.RunQuery("DELETE FROM hosts;")
	Expect(err).ToNot(HaveOccurred())
	rows.Close()
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
	res, err := req.Do(context.Background(), i.EsClient.Client)
	Expect(err).ToNot(HaveOccurred())

	err = res.Body.Close()
	Expect(err).ToNot(HaveOccurred())
}

func (i *Iteration) InsertHost(id string) {
	hbiHostFile, err := ioutil.ReadFile("./test/hbi.host.sql")
	Expect(err).ToNot(HaveOccurred())

	tmpl, err := template.New("hbiHostTemplate").Parse(string(hbiHostFile))
	Expect(err).ToNot(HaveOccurred())

	m := make(map[string]interface{})
	m["ID"] = id

	var templateBuffer bytes.Buffer
	err = tmpl.Execute(&templateBuffer, m)
	Expect(err).ToNot(HaveOccurred())
	query := strings.ReplaceAll(templateBuffer.String(), "\n", "")

	rows, err := i.DbClient.RunQuery(query)
	Expect(err).ToNot(HaveOccurred())
	rows.Close()
}

func (i *Iteration) CreateConfigMap(name string, data map[string]string) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.NamespacedName.Namespace,
		},
		Data: data,
	}

	err := test.Client.Create(context.TODO(), configMap)
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
			"url":      []byte(i.Parameters.ElasticSearchURL.String()),
			"username": []byte(i.Parameters.ElasticSearchUsername.String()),
			"password": []byte(i.Parameters.ElasticSearchPassword.String()),
		},
	}

	err := test.Client.Create(context.TODO(), secret)
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

	err := test.Client.Create(context.TODO(), secret)
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

func (i *Iteration) ExpectInitSyncReconcile() *xjoin.XJoinPipeline {
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

func (i *Iteration) CreateValidPipeline() *xjoin.XJoinPipeline {
	i.CreatePipeline()
	i.ReconcileXJoin()
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))

	return pipeline
}

func (i *Iteration) DeletePipeline(pipeline *xjoin.XJoinPipeline) {
	err := test.Client.Delete(context.Background(), pipeline)
	Expect(err).ToNot(HaveOccurred())
	i.ReconcileValidationForDeletedPipeline()
	i.ReconcileXJoinForDeletedPipeline()
}

func (i *Iteration) CreatePipeline(specs ...*xjoin.XJoinPipelineSpec) {
	var (
		ctx  = context.Background()
		spec *xjoin.XJoinPipelineSpec
	)

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
