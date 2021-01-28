package controllers

import (
	"bytes"
	"context"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/google/uuid"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"github.com/redhatinsights/xjoin-operator/test"
	"github.com/spf13/viper"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"reflect"
	"text/template"
	"time"

	//"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var resourceNamePrefix = "xjointest"

func newXJoinReconciler() *XJoinPipelineReconciler {
	return NewXJoinReconciler(
		test.Client,
		scheme.Scheme,
		logf.Log.WithName("test"),
		record.NewFakeRecorder(10))
}

func newValidationReconciler() *ValidationReconciler {
	return NewValidationReconciler(
		test.Client,
		scheme.Scheme,
		logf.Log.WithName("test-validation"),
		true,
		record.NewFakeRecorder(10))
}

func parametersToMap(parameters Parameters) map[string]interface{} {
	configReflection := reflect.ValueOf(&parameters).Elem()
	parametersMap := make(map[string]interface{})

	for i := 0; i < configReflection.NumField(); i++ {
		param := configReflection.Field(i).Interface().(Parameter)
		parametersMap[configReflection.Type().Field(i).Name] = param.Value()
	}

	return parametersMap
}

func getParameters() (Parameters, map[string]interface{}) {
	options := viper.New()
	options.SetDefault("ElasticSearchURL", "http://xjoin-elasticsearch-es-http:9200")
	options.SetDefault("ElasticSearchUsername", "test")
	options.SetDefault("ElasticSearchPassword", "test1337")
	options.SetDefault("HBIDBHost", "inventory-db")
	options.SetDefault("HBIDBPort", "5432")
	options.SetDefault("HBIDBUser", "postgres")
	options.SetDefault("HBIDBPassword", "postgres")
	options.SetDefault("HBIDBName", "test")
	options.SetDefault("ResourceNamePrefix", resourceNamePrefix)
	options.AutomaticEnv()

	xjoinConfiguration := NewXJoinConfiguration()
	err := xjoinConfiguration.ElasticSearchURL.SetValue(options.GetString("ElasticSearchURL"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.ElasticSearchUsername.SetValue(options.GetString("ElasticSearchUsername"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.ElasticSearchPassword.SetValue(options.GetString("ElasticSearchPassword"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.HBIDBHost.SetValue(options.GetString("HBIDBHost"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.HBIDBPort.SetValue(options.GetString("HBIDBPort"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.HBIDBUser.SetValue(options.GetString("HBIDBUser"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.HBIDBPassword.SetValue(options.GetString("HBIDBPassword"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.HBIDBName.SetValue(options.GetString("HBIDBName"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.ResourceNamePrefix.SetValue(options.GetString("ResourceNamePrefix"))
	Expect(err).ToNot(HaveOccurred())

	return xjoinConfiguration, parametersToMap(xjoinConfiguration)
}

func (i *TestIteration) DeleteAllHosts() {
	rows, err := i.DbClient.RunQuery("DELETE FROM hosts;")
	Expect(err).ToNot(HaveOccurred())
	rows.Close()
}

func (i *TestIteration) IndexDocument(pipelineVersion string, id string) {
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

func (i *TestIteration) InsertHost(id string) {
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

func (i *TestIteration) CreateConfigMap(name string, data map[string]string) {
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

func (i *TestIteration) CreateESSecret(name string) {
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

func (i *TestIteration) CreateDbSecret(name string) {
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

func (i *TestIteration) ExpectValidReconcile() *xjoin.XJoinPipeline {
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))
	return pipeline
}

func (i *TestIteration) ExpectNewReconcile() *xjoin.XJoinPipeline {
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_NEW))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))
	return pipeline
}

func (i *TestIteration) ExpectInitSyncReconcile() *xjoin.XJoinPipeline {
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))
	return pipeline
}

func (i *TestIteration) ExpectInvalidReconcile() *xjoin.XJoinPipeline {
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INVALID))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))
	return pipeline
}

func (i *TestIteration) CreateValidPipeline() *xjoin.XJoinPipeline {
	i.CreatePipeline()
	i.ReconcileXJoin()
	i.ReconcileValidation()
	pipeline := i.ReconcileXJoin()
	Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
	Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
	Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))

	return pipeline
}

func (i *TestIteration) CreatePipeline(specs ...*xjoin.XJoinPipelineSpec) {
	var (
		ctx  = context.Background()
		spec *xjoin.XJoinPipelineSpec
	)

	Expect(len(specs) <= 1).To(BeTrue())

	if len(specs) == 1 {
		spec = specs[0]
		specs[0].ResourceNamePrefix = &resourceNamePrefix
	} else {
		spec = &xjoin.XJoinPipelineSpec{
			ResourceNamePrefix: &resourceNamePrefix,
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

func (i *TestIteration) GetPipeline() (pipeline *xjoin.XJoinPipeline) {
	pipeline, err := utils.FetchXJoinPipeline(test.Client, i.NamespacedName)
	Expect(err).ToNot(HaveOccurred())
	return
}

func TestControllers(t *testing.T) {
	test.Setup(t, "Controllers")
}

func strToBool(str string) bool {
	b, err := strconv.ParseBool(str)
	Expect(err).ToNot(HaveOccurred())
	return b
}

func strToInt64(str string) int64 {
	i, err := strconv.ParseInt(str, 10, 64)
	Expect(err).ToNot(HaveOccurred())
	return i
}

type TestIteration struct {
	NamespacedName       types.NamespacedName
	XJoinReconciler      *XJoinPipelineReconciler
	ValidationReconciler *ValidationReconciler
	EsClient             *elasticsearch.ElasticSearch
	KafkaClient          kafka.Kafka
	Parameters           Parameters
	ParametersMap        map[string]interface{}
	DbClient             *database.Database
	Pipelines            []*xjoin.XJoinPipeline
}

func NewTestIteration() *TestIteration {
	testIteration := TestIteration{}
	return &testIteration
}

func (i *TestIteration) ReconcileXJoin() *xjoin.XJoinPipeline {
	result, err := i.XJoinReconciler.Reconcile(ctrl.Request{NamespacedName: i.NamespacedName})
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())
	return i.GetPipeline()
}

func (i *TestIteration) ReconcileValidation() *xjoin.XJoinPipeline {
	result, err := i.ValidationReconciler.Reconcile(ctrl.Request{NamespacedName: i.NamespacedName})
	Expect(err).ToNot(HaveOccurred())
	Expect(result.Requeue).To(BeFalse())
	return i.GetPipeline()
}

func (i *TestIteration) WaitForPipelineToBeValid() *xjoin.XJoinPipeline {
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

var _ = Describe("Pipeline operations", func() {
	var i *TestIteration

	BeforeEach(func() {
		i = NewTestIteration()

		i.NamespacedName = types.NamespacedName{
			Name:      "test-pipeline-01",
			Namespace: test.UniqueNamespace(resourceNamePrefix),
		}

		i.XJoinReconciler = newXJoinReconciler()
		i.ValidationReconciler = newValidationReconciler()

		es, err := elasticsearch.NewElasticSearch(
			"http://xjoin-elasticsearch-es-http:9200", "xjoin", "xjoin1337", resourceNamePrefix)
		i.EsClient = es
		Expect(err).ToNot(HaveOccurred())

		i.Parameters, i.ParametersMap = getParameters()
		i.CreateDbSecret("host-inventory-db")
		i.CreateESSecret("xjoin-elasticsearch")

		i.KafkaClient = kafka.Kafka{
			Namespace:     i.NamespacedName.Namespace,
			Client:        i.XJoinReconciler.Client,
			Parameters:    i.Parameters,
			ParametersMap: i.ParametersMap,
		}

		i.DbClient = database.NewDatabase(database.DBParams{
			Host:     i.Parameters.HBIDBHost.String(),
			Port:     i.Parameters.HBIDBPort.String(),
			User:     i.Parameters.HBIDBUser.String(),
			Password: i.Parameters.HBIDBPassword.String(),
			Name:     i.Parameters.HBIDBName.String(),
		})

		err = i.DbClient.Connect()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		//Delete any leftover ES indices
		indices, err := i.EsClient.ListIndices()
		Expect(err).ToNot(HaveOccurred())
		for _, index := range indices {
			err = i.EsClient.DeleteIndexByFullName(index)
			Expect(err).ToNot(HaveOccurred())
		}

		projects := &unstructured.UnstructuredList{}
		projects.SetKind("Namespace")
		projects.SetAPIVersion("v1")

		err = i.XJoinReconciler.Client.List(context.TODO(), projects)
		Expect(err).ToNot(HaveOccurred())

		//remove finalizers from leftover pipelines so the project can be deleted
		for _, p := range i.Pipelines {
			pipeline, err :=
				utils.FetchXJoinPipeline(test.Client, types.NamespacedName{Name: p.Name, Namespace: p.Namespace})
			Expect(err).ToNot(HaveOccurred())
			err = i.KafkaClient.DeleteConnectorsForPipelineVersion(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			err = i.KafkaClient.DeleteTopicByPipelineVersion(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			err = i.DbClient.RemoveReplicationSlotsForPipelineVersion(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			err = i.EsClient.DeleteIndex(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			i.DeleteAllHosts()
			if pipeline.DeletionTimestamp == nil {
				pipeline.ObjectMeta.Finalizers = nil
				err = i.XJoinReconciler.Client.Update(context.Background(), pipeline)
				Expect(err).ToNot(HaveOccurred())
			}
		}

		//delete leftover projects
		for _, p := range projects.Items {
			if strings.Index(p.GetName(), resourceNamePrefix) == 0 && p.GetDeletionTimestamp() == nil {
				project := &unstructured.Unstructured{}
				project.SetName(p.GetName())
				project.SetNamespace(p.GetNamespace())
				project.SetGroupVersionKind(p.GroupVersionKind())
				err = i.XJoinReconciler.Client.Delete(context.TODO(), project)
				Expect(err).ToNot(HaveOccurred())
				err = i.KafkaClient.DeleteConnectorsForPipelineVersion("1")
			}
		}
	})

	Describe("New -> InitialSync", func() {
		It("Creates a connector, ES Index, and topic for a new pipeline", func() {
			i.CreatePipeline()
			i.ReconcileXJoin()

			pipeline := i.GetPipeline()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			dbConnector, err := i.KafkaClient.GetConnector(
				i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			Expect(dbConnector.GetLabels()["strimzi.io/cluster"]).To(Equal("xjoin-kafka-connect-strimzi"))
			Expect(dbConnector.GetName()).To(Equal(resourceNamePrefix + ".db." + pipeline.Status.PipelineVersion))
			dbConnectorSpec := dbConnector.Object["spec"].(map[string]interface{})
			Expect(dbConnectorSpec["class"]).To(Equal("io.debezium.connector.postgresql.PostgresConnector"))
			Expect(dbConnectorSpec["pause"]).To(Equal(false))
			dbConnectorConfig := dbConnectorSpec["config"].(map[string]interface{})
			Expect(dbConnectorConfig["database.dbname"]).To(Equal("test"))
			Expect(dbConnectorConfig["database.password"]).To(Equal("postgres"))
			Expect(dbConnectorConfig["database.port"]).To(Equal("5432"))
			Expect(dbConnectorConfig["tasks.max"]).To(Equal("1"))
			Expect(dbConnectorConfig["database.user"]).To(Equal("postgres"))
			Expect(dbConnectorConfig["max.batch.size"]).To(Equal(int64(10)))
			Expect(dbConnectorConfig["plugin.name"]).To(Equal("pgoutput"))
			Expect(dbConnectorConfig["transforms"]).To(Equal("unwrap"))
			Expect(dbConnectorConfig["transforms.unwrap.delete.handling.mode"]).To(Equal("rewrite"))
			Expect(dbConnectorConfig["database.hostname"]).To(Equal("inventory-db"))
			Expect(dbConnectorConfig["errors.log.include.messages"]).To(Equal(true))
			Expect(dbConnectorConfig["max.queue.size"]).To(Equal(int64(1000)))
			Expect(dbConnectorConfig["poll.interval.ms"]).To(Equal(int64(100)))
			Expect(dbConnectorConfig["slot.name"]).To(Equal(resourceNamePrefix + "_" + pipeline.Status.PipelineVersion))
			Expect(dbConnectorConfig["table.whitelist"]).To(Equal("public.hosts"))
			Expect(dbConnectorConfig["database.server.name"]).To(Equal(resourceNamePrefix + "." + pipeline.Status.PipelineVersion))
			Expect(dbConnectorConfig["errors.log.enable"]).To(Equal(true))
			Expect(dbConnectorConfig["transforms.unwrap.type"]).To(Equal("io.debezium.transforms.ExtractNewRecordState"))

			esConnector, err := i.KafkaClient.GetConnector(
				i.KafkaClient.ESConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			Expect(esConnector.GetLabels()["strimzi.io/cluster"]).To(Equal("xjoin-kafka-connect-strimzi"))
			Expect(esConnector.GetName()).To(Equal(resourceNamePrefix + ".es." + pipeline.Status.PipelineVersion))
			esConnectorSpec := esConnector.Object["spec"].(map[string]interface{})
			Expect(esConnectorSpec["class"]).To(Equal("io.confluent.connect.elasticsearch.ElasticsearchSinkConnector"))
			Expect(esConnectorSpec["pause"]).To(Equal(false))
			esConnectorConfig := esConnectorSpec["config"].(map[string]interface{})
			Expect(esConnectorConfig["max.buffered.records"]).To(Equal(int64(500)))
			Expect(esConnectorConfig["transforms.deleteIf.type"]).To(Equal("com.redhat.insights.deleteifsmt.DeleteIf$Value"))
			Expect(esConnectorConfig["transforms.flattenList.sourceField"]).To(Equal("tags"))
			Expect(esConnectorConfig["errors.log.include.messages"]).To(Equal(true))
			Expect(esConnectorConfig["linger.ms"]).To(Equal(int64(100)))
			Expect(esConnectorConfig["retry.backoff.ms"]).To(Equal(int64(100)))
			Expect(esConnectorConfig["tasks.max"]).To(Equal("1"))
			Expect(esConnectorConfig["topics"]).To(Equal(resourceNamePrefix + "." + pipeline.Status.PipelineVersion + ".public.hosts"))
			Expect(esConnectorConfig["transforms.expandJSON.sourceFields"]).To(Equal("tags"))
			Expect(esConnectorConfig["transforms.flattenListString.sourceField"]).To(Equal("tags"))
			Expect(esConnectorConfig["auto.create.indices.at.start"]).To(Equal(false))
			Expect(esConnectorConfig["behavior.on.null.values"]).To(Equal("delete"))
			Expect(esConnectorConfig["connection.url"]).To(Equal("http://xjoin-elasticsearch-es-http:9200"))
			Expect(esConnectorConfig["errors.log.enable"]).To(Equal(true))
			Expect(esConnectorConfig["max.retries"]).To(Equal(int64(8)))
			Expect(esConnectorConfig["transforms.deleteIf.field"]).To(Equal("__deleted"))
			Expect(esConnectorConfig["transforms.extractKey.field"]).To(Equal("id"))
			Expect(esConnectorConfig["transforms.flattenListString.type"]).To(Equal("com.redhat.insights.flattenlistsmt.FlattenList$Value"))
			Expect(esConnectorConfig["transforms.valueToKey.type"]).To(Equal("org.apache.kafka.connect.transforms.ValueToKey"))
			Expect(esConnectorConfig["transforms"]).To(Equal("valueToKey, extractKey, expandJSON, deleteIf, flattenList, flattenListString"))
			Expect(esConnectorConfig["transforms.flattenList.mode"]).To(Equal("keys"))
			Expect(esConnectorConfig["transforms.flattenListString.encode"]).To(Equal(true))
			Expect(esConnectorConfig["transforms.flattenListString.outputField"]).To(Equal("tags_string"))
			Expect(esConnectorConfig["type.name"]).To(Equal("_doc"))
			Expect(esConnectorConfig["key.ignore"]).To(Equal("false"))
			Expect(esConnectorConfig["transforms.valueToKey.fields"]).To(Equal("id"))
			Expect(esConnectorConfig["behavior.on.malformed.documents"]).To(Equal("warn"))
			Expect(esConnectorConfig["connection.username"]).To(Equal("test"))
			Expect(esConnectorConfig["schema.ignore"]).To(Equal(true))
			Expect(esConnectorConfig["transforms.expandJSON.type"]).To(Equal("com.redhat.insights.expandjsonsmt.ExpandJSON$Value"))
			Expect(esConnectorConfig["transforms.flattenList.outputField"]).To(Equal("tags_structured"))
			Expect(esConnectorConfig["transforms.flattenList.type"]).To(Equal("com.redhat.insights.flattenlistsmt.FlattenList$Value"))
			Expect(esConnectorConfig["transforms.flattenListString.delimiterJoin"]).To(Equal("/"))
			Expect(esConnectorConfig["batch.size"]).To(Equal(int64(100)))
			Expect(esConnectorConfig["connection.password"]).To(Equal("test1337"))
			Expect(esConnectorConfig["transforms.deleteIf.value"]).To(Equal("true"))
			Expect(esConnectorConfig["transforms.extractKey.type"]).To(Equal("org.apache.kafka.connect.transforms.ExtractField$Key"))
			Expect(esConnectorConfig["transforms.flattenList.keys"]).To(Equal("namespace,key,value"))
			Expect(esConnectorConfig["transforms.flattenListString.mode"]).To(Equal("join"))
			Expect(esConnectorConfig["max.in.flight.requests"]).To(Equal(int64(1)))

			exists, err := i.EsClient.IndexExists(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			aliases, err := i.EsClient.GetCurrentIndicesWithAlias(*pipeline.Spec.ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(aliases).To(BeEmpty())

			topics, err := i.KafkaClient.ListTopicNamesForPipelineVersion(pipeline.Status.PipelineVersion)
			Expect(topics).To(ContainElement(i.KafkaClient.TopicName(pipeline.Status.PipelineVersion)))
		})

		It("Considers configmap configuration", func() {
			i.CreatePipeline()

			cm := map[string]string{
				"connect.cluster":                                     "test.connect.cluster",
				"kafka.cluster":                                       "test.kafka.cluster",
				"debezium.connector.tasks.max":                        "-1",
				"debezium.connector.max.batch.size":                   "-2",
				"debezium.connector.max.queue.size":                   "-3",
				"debezium.connector.poll.interval.ms":                 "-4",
				"debezium.connector.errors.log.enable":                "false",
				"elasticsearch.connector.tasks.max":                   "-5",
				"elasticsearch.connector.batch.size":                  "-6",
				"elasticsearch.connector.max.in.flight.requests":      "-7",
				"elasticsearch.connector.errors.log.enable":           "false",
				"elasticsearch.connector.errors.log.include.messages": "false",
				"elasticsearch.connector.max.retries":                 "-8",
				"elasticsearch.connector.retry.backoff.ms":            "-9",
				"elasticsearch.connector.max.buffered.records":        "-10",
				"elasticsearch.connector.linger.ms":                   "-11",
				"standard.interval":                                   "-12",
				"validation.percentage.threshold":                     "-13",
				"init.validation.percentage.threshold":                "-14",
				"validation.attempts.threshold":                       "-15",
				"init.validation.attempts.threshold":                  "-16",
				"validation.interval":                                 "-17",
				"init.validation.interval":                            "-18",
			}

			i.CreateConfigMap("xjoin", cm)
			i.ReconcileXJoin()

			pipeline := i.GetPipeline()
			esConnector, err := i.KafkaClient.GetConnector(
				i.KafkaClient.ESConnectorName(pipeline.Status.PipelineVersion))

			Expect(err).ToNot(HaveOccurred())
			Expect(esConnector.GetLabels()["strimzi.io/cluster"]).To(Equal(cm["connect.cluster"]))
			Expect(esConnector.GetName()).To(Equal(resourceNamePrefix + ".es." + pipeline.Status.PipelineVersion))
			esConnectorSpec := esConnector.Object["spec"].(map[string]interface{})
			esConnectorConfig := esConnectorSpec["config"].(map[string]interface{})
			Expect(esConnectorConfig["tasks.max"]).To(Equal(cm["elasticsearch.connector.tasks.max"]))
			Expect(esConnectorConfig["topics"]).To(Equal(resourceNamePrefix + "." + pipeline.Status.PipelineVersion + ".public.hosts"))
			Expect(esConnectorConfig["batch.size"]).To(Equal(strToInt64(cm["elasticsearch.connector.batch.size"])))
			Expect(esConnectorConfig["max.in.flight.requests"]).To(Equal(strToInt64(cm["elasticsearch.connector.max.in.flight.requests"])))
			Expect(esConnectorConfig["errors.log.enable"]).To(Equal(strToBool(cm["elasticsearch.connector.errors.log.enable"])))
			Expect(esConnectorConfig["max.retries"]).To(Equal(strToInt64(cm["elasticsearch.connector.max.retries"])))
			Expect(esConnectorConfig["retry.backoff.ms"]).To(Equal(strToInt64(cm["elasticsearch.connector.retry.backoff.ms"])))
			Expect(esConnectorConfig["max.buffered.records"]).To(Equal(strToInt64(cm["elasticsearch.connector.max.buffered.records"])))
			Expect(esConnectorConfig["linger.ms"]).To(Equal(strToInt64(cm["elasticsearch.connector.linger.ms"])))

			dbConnector, err := i.KafkaClient.GetConnector(
				i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion))

			Expect(err).ToNot(HaveOccurred())
			Expect(dbConnector.GetLabels()["strimzi.io/cluster"]).To(Equal(cm["connect.cluster"]))
			Expect(dbConnector.GetName()).To(Equal(resourceNamePrefix + ".db." + pipeline.Status.PipelineVersion))
			dbConnectorSpec := dbConnector.Object["spec"].(map[string]interface{})
			dbConnectorConfig := dbConnectorSpec["config"].(map[string]interface{})
			Expect(dbConnectorConfig["tasks.max"]).To(Equal(cm["debezium.connector.tasks.max"]))
			Expect(dbConnectorConfig["max.batch.size"]).To(Equal(strToInt64(cm["debezium.connector.max.batch.size"])))
			Expect(dbConnectorConfig["max.queue.size"]).To(Equal(strToInt64(cm["debezium.connector.max.queue.size"])))
			Expect(dbConnectorConfig["poll.interval.ms"]).To(Equal(strToInt64(cm["debezium.connector.poll.interval.ms"])))
			Expect(dbConnectorConfig["errors.log.enable"]).To(Equal(strToBool(cm["debezium.connector.errors.log.enable"])))
		})

		It("Considers db secret name configuration", func() {
			hbiDBSecret, err := utils.FetchSecret(
				test.Client, i.NamespacedName.Namespace, i.Parameters.HBIDBSecretName.String())
			Expect(err).ToNot(HaveOccurred())
			err = test.Client.Delete(context.TODO(), hbiDBSecret)
			Expect(err).ToNot(HaveOccurred())

			secretName := "test-hbi-db-secret"
			i.CreateDbSecret(secretName)

			i.CreatePipeline(&xjoin.XJoinPipelineSpec{HBIDBSecretName: &secretName})
			i.ReconcileXJoin() //this will fail if the secret is missing
		})

		It("Considers es secret name configuration", func() {
			elasticSearchSecret, err := utils.FetchSecret(test.Client, i.NamespacedName.Namespace, i.Parameters.ElasticSearchSecretName.String())
			Expect(err).ToNot(HaveOccurred())
			err = test.Client.Delete(context.TODO(), elasticSearchSecret)
			Expect(err).ToNot(HaveOccurred())

			secretName := "test-elasticsearch-secret"
			i.CreateESSecret(secretName)

			i.CreatePipeline(&xjoin.XJoinPipelineSpec{ElasticSearchSecretName: &secretName})
			i.ReconcileXJoin() //this will fail if the secret is missing
		})

		It("Removes stale connectors", func() {
			_, err := i.KafkaClient.CreateDebeziumConnector("1", false)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.KafkaClient.CreateDebeziumConnector("2", false)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.KafkaClient.CreateESConnector("1", false)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.KafkaClient.CreateESConnector("2", false)
			Expect(err).ToNot(HaveOccurred())

			connectors, err := i.KafkaClient.ListConnectors()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(connectors.Items)).To(Equal(4))

			i.CreatePipeline()
			i.ReconcileXJoin()
			connectors, err = i.KafkaClient.ListConnectors()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(connectors.Items)).To(Equal(2))
		})

		It("Removes stale indices", func() {
			err := i.EsClient.CreateIndex("1")
			Expect(err).ToNot(HaveOccurred())
			err = i.EsClient.CreateIndex("2")
			Expect(err).ToNot(HaveOccurred())

			indices, err := i.EsClient.ListIndices()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(indices)).To(Equal(2))

			i.CreatePipeline()
			i.ReconcileXJoin()
			indices, err = i.EsClient.ListIndices()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(indices)).To(Equal(1))
		})

		It("Removes stale topics", func() {
			err := i.KafkaClient.CreateTopic("1")
			Expect(err).ToNot(HaveOccurred())
			err = i.KafkaClient.CreateTopic("2")
			Expect(err).ToNot(HaveOccurred())

			topics, err := i.KafkaClient.ListTopicNames()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(topics)).To(Equal(2))

			i.CreatePipeline()
			i.ReconcileXJoin()

			topics, err = i.KafkaClient.ListTopicNames()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(topics)).To(Equal(1))
		})

		It("Removes stale replication slots", func() {
			slot1 := resourceNamePrefix + "_1"
			slot2 := resourceNamePrefix + "_2"
			err := i.DbClient.RemoveReplicationSlotsForPrefix(resourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			err = i.DbClient.CreateReplicationSlot(slot1)
			Expect(err).ToNot(HaveOccurred())
			err = i.DbClient.CreateReplicationSlot(slot2)
			Expect(err).ToNot(HaveOccurred())

			slots, err := i.DbClient.ListReplicationSlots(resourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(slots).To(ContainElements(slot1, slot2))

			i.CreatePipeline()
			i.ReconcileXJoin()

			slots, err = i.DbClient.ListReplicationSlots(resourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())

			Expect(slots).ToNot(ContainElements(slot1, slot2))
		})
	})

	Describe("InitialSync -> Valid", func() {
		It("Creates the elasticsearch alias", func() {
			i.CreatePipeline()
			i.ReconcileXJoin()
			pipeline := i.GetPipeline()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			i.ReconcileValidation()
			pipeline = i.ReconcileXJoin()

			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))
		})

		It("Triggers refresh if pipeline fails to become valid for too long", func() {
			hostId, err := uuid.NewUUID()
			Expect(err).ToNot(HaveOccurred())

			i.CreatePipeline()

			cm := map[string]string{
				"init.validation.attempts.threshold": "2",
			}

			i.CreateConfigMap("xjoin", cm)
			i.ReconcileXJoin()
			pipeline := i.GetPipeline()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			err = i.KafkaClient.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			i.InsertHost(hostId.String())

			i.ReconcileValidation()
			pipeline = i.ReconcileXJoin()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))

			i.ReconcileValidation()
			pipeline = i.ReconcileXJoin()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_NEW))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))
			Expect(pipeline.Status.PipelineVersion).To(Equal(""))

			i.ReconcileValidation()
			pipeline = i.ReconcileXJoin()
			Expect(pipeline.Status.PipelineVersion).ToNot(Equal(""))
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			i.IndexDocument(pipeline.Status.PipelineVersion, hostId.String())

			i.ReconcileValidation()
			pipeline = i.ReconcileXJoin()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))
		})
	})

	Describe("Invalid -> New", func() {
		Context("In a refresh", func() {
			It("Keeps the old table active until the new one is valid", func() {
				hostId, err := uuid.NewUUID()
				Expect(err).ToNot(HaveOccurred())

				cm := map[string]string{
					"validation.attempts.threshold": "2",
				}
				i.CreateConfigMap("xjoin", cm)

				pipeline := i.CreateValidPipeline()
				activeIndex := pipeline.Status.ActiveIndexName

				err = i.KafkaClient.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
				Expect(err).ToNot(HaveOccurred())
				i.InsertHost(hostId.String())

				pipeline = i.ExpectInvalidReconcile()
				Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

				pipeline = i.ExpectNewReconcile()
				Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

				i.InsertHost(hostId.String())

				pipeline = i.ExpectInitSyncReconcile()
				Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

				i.IndexDocument(pipeline.Status.PipelineVersion, hostId.String())

				pipeline = i.ExpectValidReconcile()
				Expect(pipeline.Status.ActiveIndexName).ToNot(Equal(activeIndex))
			})
		})
	})

	Describe("Valid -> New", func() {
		It("Triggers refresh if configmap is created", func() {
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName

			clusterName := "invalid.cluster"
			cm := map[string]string{
				"connect.cluster": clusterName,
			}
			i.CreateConfigMap("xjoin", cm)

			pipeline = i.ExpectInitSyncReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			connector, err := i.KafkaClient.GetConnector(i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			Expect(connector.GetLabels()["strimzi.io/cluster"]).To(Equal(clusterName))
		})

		It("Triggers refresh if configmap changes", func() {
			cm := map[string]string{
				"connect.cluster": i.Parameters.ConnectCluster.String(),
			}
			i.CreateConfigMap("xjoin", cm)
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName

			configMap, err := utils.FetchConfigMap(test.Client, i.NamespacedName.Namespace, "xjoin")
			Expect(err).ToNot(HaveOccurred())
			updatedClusterName := "invalid.cluster"
			cm["connect.cluster"] = updatedClusterName
			configMap.Data = cm
			err = test.Client.Update(context.TODO(), configMap)
			Expect(err).ToNot(HaveOccurred())

			pipeline = i.ExpectInitSyncReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			connector, err := i.KafkaClient.GetConnector(i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			Expect(connector.GetLabels()["strimzi.io/cluster"]).To(Equal(updatedClusterName))
		})

		It("Triggers refresh if database secret changes", func() {
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName

			secret, err := utils.FetchSecret(test.Client, i.NamespacedName.Namespace, i.Parameters.HBIDBSecretName.String())
			Expect(err).ToNot(HaveOccurred())

			tempUser := "tempuser"
			tempPassword := "temppassword"
			_, _ = i.DbClient.Exec( //allow this to fail when the user already exists
				fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s' IN ROLE insights;", tempUser, tempPassword))
			secret.Data["db.user"] = []byte(tempUser)
			secret.Data["db.password"] = []byte(tempPassword)
			err = test.Client.Update(context.TODO(), secret)

			pipeline = i.ExpectInitSyncReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			connector, err := i.KafkaClient.GetConnector(i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			connectorSpec := connector.Object["spec"].(map[string]interface{})
			connectorConfig := connectorSpec["config"].(map[string]interface{})
			Expect(connectorConfig["database.user"]).To(Equal(tempUser))
			Expect(connectorConfig["database.password"]).To(Equal(tempPassword))
		})

		FIt("Triggers refresh if elasticsearch secret changes", func() {
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName

			secret, err := utils.FetchSecret(test.Client, i.NamespacedName.Namespace, i.Parameters.ElasticSearchSecretName.String())
			Expect(err).ToNot(HaveOccurred())

			//change the secret hash by adding a new field
			secret.Data["newfield"] = []byte("value")
			err = test.Client.Update(context.TODO(), secret)

			pipeline = i.ExpectInitSyncReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			connector, err := i.KafkaClient.GetConnector(i.KafkaClient.ESConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			connectorSpec := connector.Object["spec"].(map[string]interface{})
			connectorConfig := connectorSpec["config"].(map[string]interface{})
			Expect(connectorConfig["connection.username"]).To(Equal("test"))
			Expect(connectorConfig["connection.password"]).To(Equal("test1337"))
			Expect(connectorConfig["connection.url"]).To(Equal("http://xjoin-elasticsearch-es-http:9200"))
		})

		It("Triggers refresh if index disappears", func() {
		})

		It("Triggers refresh if elasticsearch connector disappears", func() {
		})

		It("Triggers refresh if database connector disappears", func() {
		})

		It("Triggers refresh if database connector configuration disagrees", func() {
		})

		It("Triggers refresh if elasticsearch connector configuration disagrees", func() {
		})

		It("Triggers refresh if connect cluster changes", func() {
		})

		It("Triggers refresh if MaxAge changes", func() {
		})
	})

	Describe("-> Removed", func() {
		It("Artifacts removed when initializing pipeline is removed", func() {
		})

		It("Artifacts removed when valid pipeline is removed", func() {
		})
	})

	Describe("Failures", func() {
		It("Fails if App DB secret is missing", func() {
		})

		It("Fails if App DB secret is misconfigured", func() {
		})

		It("Fails if the configmap is misconfigured", func() {

		})

		It("Fails if DB table cannot be created", func() {
		})

		It("Fails if inventory.hosts view cannot be created", func() {
		})
	})
})
