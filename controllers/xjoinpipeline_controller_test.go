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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var resourceNamePrefix = "test"

func newXJoinReconciler() *XJoinPipelineReconciler {
	return NewXJoinReconciler(
		test.Client,
		scheme.Scheme,
		logf.Log.WithName("test"),
		record.NewFakeRecorder(10))
}

func getDBParams() DBParams {
	options := viper.New()
	options.SetDefault("DBHostHBI", "localhost")
	options.SetDefault("DBPort", "5432")
	options.SetDefault("DBUser", "postgres")
	options.SetDefault("DBPass", "postgres")
	options.SetDefault("DBName", "test")
	options.AutomaticEnv()

	return DBParams{
		Host:     options.GetString("DBHostHBI"),
		Port:     options.GetString("DBPort"),
		Name:     options.GetString("DBName"),
		User:     options.GetString("DBUser"),
		Password: options.GetString("DBPass"),
	}
}

func createDbSecret(namespace string, name string, params DBParams) {
	secret := &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"db.host":     []byte(params.Host),
			"db.port":     []byte(params.Port),
			"db.name":     []byte(params.Name),
			"db.user":     []byte(params.User),
			"db.password": []byte(params.Password),
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
		namePrefix := "test"
		spec = &xjoin.XJoinPipelineSpec{
			ResourceNamePrefix: &namePrefix,
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

var _ = Describe("Pipeline operations", func() {
	var (
		namespacedName types.NamespacedName
		r              *XJoinPipelineReconciler
		es             *elasticsearch.ElasticSearch
		dbParams       DBParams
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

		dbParams = getDBParams()
		createDbSecret(namespacedName.Namespace, "host-inventory-db", dbParams)
	})

	AfterEach(func() {
		//Delete any leftover ES indices
		indices, err := es.ListIndices(resourceNamePrefix)
		Expect(err).ToNot(HaveOccurred())
		for _, index := range indices {
			err = es.DeleteIndexByFullName(index)
			Expect(err).ToNot(HaveOccurred())
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
			Expect(dbConnector.GetName()).To(Equal("test.db." + pipeline.Status.PipelineVersion))

			esConnector, err := kafka.GetConnector(
				test.Client, kafka.ESConnectorName(*pipeline.Spec.ResourceNamePrefix, pipeline.Status.PipelineVersion), namespacedName.Namespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(esConnector.GetLabels()["strimzi.io/cluster"]).To(Equal("xjoin-kafka-connect-strimzi"))
			Expect(esConnector.GetName()).To(Equal("test.es." + pipeline.Status.PipelineVersion))

			exists, err := es.IndexExists(*pipeline.Spec.ResourceNamePrefix, pipeline.Status.PipelineVersion)

			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})
	})
})
