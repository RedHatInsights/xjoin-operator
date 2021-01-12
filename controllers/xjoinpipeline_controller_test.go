package controllers

import (
	"context"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"github.com/redhatinsights/xjoin-operator/test"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func getXJoinConfig() Parameters {
	options := viper.New()
	options.SetDefault("ElasticSearchURL", "http://localhost:9200")
	options.SetDefault("ElasticSearchUsername", "test")
	options.SetDefault("ElasticSearchPassword", "test1337")
	options.SetDefault("HBIDBHost", "localhost")
	options.SetDefault("HBIDBPort", "5432")
	options.SetDefault("HBIDBUser", "postgres")
	options.SetDefault("HBIDBPassword", "postgres")
	options.SetDefault("HBIDBName", "test")
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

	return xjoinConfiguration
}

func createConfigMap(namespace string, name string, data map[string]string) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	err := test.Client.Create(context.TODO(), configMap)
	Expect(err).ToNot(HaveOccurred())
}

func createESSecret(namespace string, name string, params Parameters) {
	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"url":      []byte(params.ElasticSearchURL.String()),
			"username": []byte(params.ElasticSearchUsername.String()),
			"password": []byte(params.ElasticSearchPassword.String()),
		},
	}

	err := test.Client.Create(context.TODO(), secret)
	Expect(err).ToNot(HaveOccurred())
}

func createDbSecret(namespace string, name string, params Parameters) {
	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"db.host":     []byte(params.HBIDBHost.String()),
			"db.port":     []byte(params.HBIDBPort.String()),
			"db.name":     []byte(params.HBIDBName.String()),
			"db.user":     []byte(params.HBIDBUser.String()),
			"db.password": []byte(params.HBIDBPassword.String()),
		},
	}

	err := test.Client.Create(context.TODO(), secret)
	Expect(err).ToNot(HaveOccurred())
}

func createPipeline(namespacedName types.NamespacedName, specs ...*xjoin.XJoinPipelineSpec) {
	var (
		ctx  = context.Background()
		spec *xjoin.XJoinPipelineSpec
	)

	Expect(len(specs) <= 1).To(BeTrue())

	if len(specs) == 1 {
		spec = specs[0]
	} else {
		spec = &xjoin.XJoinPipelineSpec{
			ResourceNamePrefix: &resourceNamePrefix,
		}
	}

	pipeline := xjoin.XJoinPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
		},
		Spec: *spec,
	}

	err := test.Client.Create(ctx, &pipeline)
	Expect(err).ToNot(HaveOccurred())
}

func getPipeline(namespacedName types.NamespacedName) (pipeline *xjoin.XJoinPipeline) {
	pipeline, err := utils.FetchXJoinPipeline(test.Client, namespacedName)
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

var _ = Describe("Pipeline operations", func() {
	var (
		namespacedName types.NamespacedName
		r              *XJoinPipelineReconciler
		es             *elasticsearch.ElasticSearch
	)

	var reconcile = func() (result ctrl.Result) {
		result, err := r.Reconcile(ctrl.Request{NamespacedName: namespacedName})
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeFalse())
		return
	}

	BeforeEach(func() {
		namespacedName = types.NamespacedName{
			Name:      "test-pipeline-01",
			Namespace: test.UniqueNamespace(),
		}

		r = newXJoinReconciler()

		esClient, err := elasticsearch.NewElasticSearch(
			"http://localhost:9200", "xjoin", "xjoin1337")
		es = esClient
		Expect(err).ToNot(HaveOccurred())

		xjoinConfig := getXJoinConfig()
		createDbSecret(namespacedName.Namespace, "host-inventory-db", xjoinConfig)
		createESSecret(namespacedName.Namespace, "xjoin-elasticsearch", xjoinConfig)
	})

	AfterEach(func() {
		//Delete any leftover ES indices
		indices, err := es.ListIndices(resourceNamePrefix)
		Expect(err).ToNot(HaveOccurred())
		for _, index := range indices {
			err = es.DeleteIndexByFullName(index)
			Expect(err).ToNot(HaveOccurred())
		}

		projects := &unstructured.UnstructuredList{}
		projects.SetKind("Namespace")
		projects.SetAPIVersion("v1")

		err = r.Client.List(context.TODO(), projects)
		Expect(err).ToNot(HaveOccurred())

		for _, p := range projects.Items {
			if strings.Index(p.GetName(), resourceNamePrefix) == 0 {
				project := &unstructured.Unstructured{}
				project.SetName(p.GetName())
				project.SetNamespace(p.GetNamespace())
				project.SetGroupVersionKind(p.GroupVersionKind())
				err = r.Client.Delete(context.TODO(), project)
				Expect(err).ToNot(HaveOccurred())
			}
		}
	})

	Describe("New -> InitialSync", func() {
		It("Creates a connector and ES Index for a new pipeline", func() {
			createPipeline(namespacedName)
			reconcile()

			pipeline := getPipeline(namespacedName)
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			dbConnector, err := kafka.GetConnector(
				test.Client, kafka.DebeziumConnectorName(*pipeline.Spec.ResourceNamePrefix, pipeline.Status.PipelineVersion), namespacedName.Namespace)
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
			Expect(dbConnectorConfig["database.hostname"]).To(Equal("localhost"))
			Expect(dbConnectorConfig["errors.log.include.messages"]).To(Equal(true))
			Expect(dbConnectorConfig["max.queue.size"]).To(Equal(int64(1000)))
			Expect(dbConnectorConfig["poll.interval.ms"]).To(Equal(int64(100)))
			Expect(dbConnectorConfig["slot.name"]).To(Equal("xjoin_inventory_" + pipeline.Status.PipelineVersion))
			Expect(dbConnectorConfig["table.whitelist"]).To(Equal("public.hosts"))
			Expect(dbConnectorConfig["database.server.name"]).To(Equal("xjoin.inventory." + pipeline.Status.PipelineVersion))
			Expect(dbConnectorConfig["errors.log.enable"]).To(Equal(true))
			Expect(dbConnectorConfig["transforms.unwrap.type"]).To(Equal("io.debezium.transforms.ExtractNewRecordState"))

			esConnector, err := kafka.GetConnector(
				test.Client, kafka.ESConnectorName(*pipeline.Spec.ResourceNamePrefix, pipeline.Status.PipelineVersion), namespacedName.Namespace)
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
			Expect(esConnectorConfig["tasks.max"]).To(Equal("50"))
			Expect(esConnectorConfig["topics"]).To(Equal("xjoin.inventory." + pipeline.Status.PipelineVersion + ".public.hosts"))
			Expect(esConnectorConfig["transforms.expandJSON.sourceFields"]).To(Equal("tags"))
			Expect(esConnectorConfig["transforms.flattenListString.sourceField"]).To(Equal("tags"))
			Expect(esConnectorConfig["transforms.renameTopic.type"]).To(Equal("org.apache.kafka.connect.transforms.RegexRouter"))
			Expect(esConnectorConfig["auto.create.indices.at.start"]).To(Equal(false))
			Expect(esConnectorConfig["behavior.on.null.values"]).To(Equal("delete"))
			Expect(esConnectorConfig["connection.url"]).To(Equal("http://localhost:9200"))
			Expect(esConnectorConfig["errors.log.enable"]).To(Equal(true))
			Expect(esConnectorConfig["max.retries"]).To(Equal(int64(8)))
			Expect(esConnectorConfig["transforms.deleteIf.field"]).To(Equal("__deleted"))
			Expect(esConnectorConfig["transforms.extractKey.field"]).To(Equal("id"))
			Expect(esConnectorConfig["transforms.flattenListString.type"]).To(Equal("com.redhat.insights.flattenlistsmt.FlattenList$Value"))
			Expect(esConnectorConfig["transforms.valueToKey.type"]).To(Equal("org.apache.kafka.connect.transforms.ValueToKey"))
			Expect(esConnectorConfig["transforms"]).To(Equal("valueToKey, extractKey, expandJSON, deleteIf, flattenList, flattenListString, renameTopic"))
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
			Expect(esConnectorConfig["transforms.renameTopic.regex"]).To(Equal("xjoin\\.inventory\\." + pipeline.Status.PipelineVersion + ".public\\.hosts"))
			Expect(esConnectorConfig["transforms.renameTopic.replacement"]).To(Equal("xjoin.inventory.hosts." + pipeline.Status.PipelineVersion))
			Expect(esConnectorConfig["max.in.flight.requests"]).To(Equal(int64(1)))

			exists, err := es.IndexExists(*pipeline.Spec.ResourceNamePrefix, pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			aliases, err := es.GetCurrentIndicesWithAlias(*pipeline.Spec.ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(aliases).To(BeEmpty())
		})

		It("Considers configmap configuration", func() {
			createPipeline(namespacedName)

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

			createConfigMap(namespacedName.Namespace, "xjoin", cm)
			reconcile()

			pipeline := getPipeline(namespacedName)
			esConnector, err := kafka.GetConnector(
				test.Client, kafka.ESConnectorName(*pipeline.Spec.ResourceNamePrefix, pipeline.Status.PipelineVersion), namespacedName.Namespace)

			Expect(err).ToNot(HaveOccurred())
			Expect(esConnector.GetLabels()["strimzi.io/cluster"]).To(Equal(cm["connect.cluster"]))
			Expect(esConnector.GetName()).To(Equal(resourceNamePrefix + ".es." + pipeline.Status.PipelineVersion))
			esConnectorSpec := esConnector.Object["spec"].(map[string]interface{})
			esConnectorConfig := esConnectorSpec["config"].(map[string]interface{})
			Expect(esConnectorConfig["tasks.max"]).To(Equal(cm["elasticsearch.connector.tasks.max"]))
			Expect(esConnectorConfig["topics"]).To(Equal("xjoin.inventory." + pipeline.Status.PipelineVersion + ".public.hosts"))
			Expect(esConnectorConfig["batch.size"]).To(Equal(strToInt64(cm["elasticsearch.connector.batch.size"])))
			Expect(esConnectorConfig["max.in.flight.requests"]).To(Equal(strToInt64(cm["elasticsearch.connector.max.in.flight.requests"])))
			Expect(esConnectorConfig["errors.log.enable"]).To(Equal(strToBool(cm["elasticsearch.connector.errors.log.enable"])))
			Expect(esConnectorConfig["max.retries"]).To(Equal(strToInt64(cm["elasticsearch.connector.max.retries"])))
			Expect(esConnectorConfig["retry.backoff.ms"]).To(Equal(strToInt64(cm["elasticsearch.connector.retry.backoff.ms"])))
			Expect(esConnectorConfig["max.buffered.records"]).To(Equal(strToInt64(cm["elasticsearch.connector.max.buffered.records"])))
			Expect(esConnectorConfig["linger.ms"]).To(Equal(strToInt64(cm["elasticsearch.connector.linger.ms"])))

			dbConnector, err := kafka.GetConnector(
				test.Client, kafka.DebeziumConnectorName(*pipeline.Spec.ResourceNamePrefix, pipeline.Status.PipelineVersion), namespacedName.Namespace)

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

	})
})
