package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/RedHatInsights/strimzi-client-go/apis/kafka.strimzi.io/v1beta2"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("XJoinDataSourcePipeline", func() {
	var namespace string

	var newXJoinDataSourcePipelineReconciler = func() *XJoinDataSourcePipelineReconciler {
		return NewXJoinDataSourcePipelineReconciler(
			k8sClient,
			scheme.Scheme,
			testLogger,
			record.NewFakeRecorder(10),
			namespace,
			true)
	}

	var createValidDataSourcePipeline = func(name string) {
		ctx := context.Background()

		datasourceSpec := v1alpha1.XJoinDataSourcePipelineSpec{
			Name:             name,
			Version:          "1234",
			AvroSchema:       "{}",
			DatabaseHostname: &v1alpha1.StringOrSecretParameter{Value: "dbHost"},
			DatabasePort:     &v1alpha1.StringOrSecretParameter{Value: "8080"},
			DatabaseUsername: &v1alpha1.StringOrSecretParameter{Value: "dbUsername"},
			DatabasePassword: &v1alpha1.StringOrSecretParameter{Value: "dbPassword"},
			DatabaseName:     &v1alpha1.StringOrSecretParameter{Value: "dbName"},
			DatabaseTable:    &v1alpha1.StringOrSecretParameter{Value: "dbTable"},
			Pause:            false,
		}

		datasource := &v1alpha1.XJoinDataSourcePipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: datasourceSpec,
			TypeMeta: metav1.TypeMeta{
				APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
				Kind:       "XJoinDataSourcePipeline",
			},
		}

		Expect(k8sClient.Create(ctx, datasource)).Should(Succeed())

		//validate datasource spec is created correctly
		datasourceLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
		createdDataSourcePipeline := &v1alpha1.XJoinDataSourcePipeline{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, datasourceLookupKey, createdDataSourcePipeline)
			if err != nil {
				return false
			}
			return true
		}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())

		Expect(createdDataSourcePipeline.Spec.Name).Should(Equal(name))
		Expect(createdDataSourcePipeline.Spec.Version).Should(Equal("1234"))
		Expect(createdDataSourcePipeline.Spec.Pause).Should(Equal(false))
		Expect(createdDataSourcePipeline.Spec.AvroSchema).Should(Equal("{}"))
		Expect(createdDataSourcePipeline.Spec.DatabaseHostname).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbHost"}))
		Expect(createdDataSourcePipeline.Spec.DatabasePort).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "8080"}))
		Expect(createdDataSourcePipeline.Spec.DatabaseUsername).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbUsername"}))
		Expect(createdDataSourcePipeline.Spec.DatabasePassword).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbPassword"}))
		Expect(createdDataSourcePipeline.Spec.DatabaseName).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbName"}))
		Expect(createdDataSourcePipeline.Spec.DatabaseTable).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbTable"}))
	}

	var reconcileDataSourcePipeline = func(name string) v1alpha1.XJoinDataSourcePipeline {
		ctx := context.Background()

		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline.test-data-source-pipeline.1234-value/versions/1",
			httpmock.NewStringResponder(404, `{"message":"No version '1' found for artifact with ID 'xjoindatasourcepipeline.test-data-source-pipeline.1234-value' in group 'null'.","error_code":40402}`).Times(1))

		httpmock.RegisterResponder(
			"POST",
			"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline.test-data-source-pipeline.1234-value/versions",
			httpmock.NewStringResponder(200, `{"createdBy":"","createdOn":"2022-07-27T17:28:11+0000","modifiedBy":"","modifiedOn":"2022-07-27T17:28:11+0000","id":1,"version":1,"type":"AVRO","globalId":1,"state":"ENABLED","groupId":"null","contentId":1,"references":[]}`))

		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline.test-data-source-pipeline.1234-value/versions/latest",
			httpmock.NewStringResponder(200, `{"schema":"{\"name\":\"Value\",\"namespace\":\"xjoindatasourcepipeline.test-data-source-pipeline\"}","schemaType":"AVRO","references":[]}`))

		xjoinDataSourcePipelineReconciler := newXJoinDataSourcePipelineReconciler()

		datasourceLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
		createdDataSourcePipeline := &v1alpha1.XJoinDataSourcePipeline{}

		result, err := xjoinDataSourcePipelineReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: datasourceLookupKey})
		checkError(err)
		Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

		Eventually(func() bool {
			err := k8sClient.Get(ctx, datasourceLookupKey, createdDataSourcePipeline)
			if err != nil {
				return false
			}
			return true
		}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())

		return *createdDataSourcePipeline
	}

	BeforeEach(func() {
		httpmock.Activate()
		httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

		var err error
		namespace, err = NewNamespace()
		checkError(err)
	})

	AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	Context("Reconcile", func() {
		It("Should add a finalizer to the datasourcepipeline", func() {
			dataSourcePipelineName := "test-data-source-pipeline"

			createValidDataSourcePipeline(dataSourcePipelineName)
			createdDataSourcePipeline := reconcileDataSourcePipeline(dataSourcePipelineName)
			Expect(createdDataSourcePipeline.Finalizers).To(HaveLen(1))
			Expect(createdDataSourcePipeline.Finalizers).To(ContainElement("finalizer.xjoin.datasourcepipeline.cloud.redhat.com"))
		})

		It("Creates a Debezium Kafka Connector", func() {
			dataSourcePipelineName := "test-data-source-pipeline"
			createValidDataSourcePipeline(dataSourcePipelineName)
			reconcileDataSourcePipeline(dataSourcePipelineName)

			ctx := context.Background()
			debeziumConnectorName := "xjoindatasourcepipeline.test-data-source-pipeline.1234"
			debeziumConnectorLookupKey := types.NamespacedName{Name: debeziumConnectorName, Namespace: namespace}
			debeziumConnector := &v1beta2.KafkaConnector{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, debeziumConnectorLookupKey, debeziumConnector)
				if err != nil {
					return false
				}
				return true
			}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())

			debeziumClass := "io.debezium.connector.postgresql.PostgresConnector"
			debeziumPause := false
			debeziumTasksMax := int32(1)

			Expect(debeziumConnector.Name).To(Equal(debeziumConnectorName))
			Expect(debeziumConnector.GetLabels()).To(Equal(map[string]string{"strimzi.io/cluster": "connect"}))
			Expect(debeziumConnector.Namespace).To(Equal(namespace))
			Expect(debeziumConnector.Spec.Class).To(Equal(&debeziumClass))
			Expect(debeziumConnector.Spec.Pause).To(Equal(&debeziumPause))
			Expect(debeziumConnector.Spec.TasksMax).To(Equal(&debeziumTasksMax))

			//config comparison
			expectedDebeziumConfig := LoadExpectedKafkaResourceConfig("./test/data/kafka/debezium_config.json")
			actualDebeziumConfig := bytes.NewBuffer([]byte{})
			err := json.Compact(actualDebeziumConfig, debeziumConnector.Spec.Config.Raw)
			checkError(err)
			Expect(actualDebeziumConfig).To(Equal(expectedDebeziumConfig))
		})

		It("Creates an Avro Schema", func() {
			dataSourcePipelineName := "test-data-source-pipeline"
			createValidDataSourcePipeline(dataSourcePipelineName)
			reconcileDataSourcePipeline(dataSourcePipelineName)

			//TODO validate the body of the request is correct
			//validates the correct API calls were made
			info := httpmock.GetCallCountInfo()
			count := info["POST http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline.test-data-source-pipeline.1234-value/versions"]
			Expect(count).To(Equal(1))

			count = info["GET http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline.test-data-source-pipeline.1234-value/versions/1"]
			Expect(count).To(Equal(1))

			count = info["GET http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline.test-data-source-pipeline.1234-value/versions/latest"]
			Expect(count).To(Equal(1))
		})

		It("Creates a Kafka Topic", func() {
			dataSourcePipelineName := "test-data-source-pipeline"
			createValidDataSourcePipeline(dataSourcePipelineName)
			reconcileDataSourcePipeline(dataSourcePipelineName)

			ctx := context.Background()
			kafkaTopicName := "xjoindatasourcepipeline.test-data-source-pipeline.1234"
			kafkaTopicLookupKey := types.NamespacedName{Name: kafkaTopicName, Namespace: namespace}
			kafkaTopic := &v1beta2.KafkaTopic{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, kafkaTopicLookupKey, kafkaTopic)
				if err != nil {
					return false
				}
				return true
			}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())

			kafkaTopicPartitions := int32(1)
			kafkaTopicReplicas := int32(1)
			Expect(kafkaTopic.Name).To(Equal(kafkaTopicName))
			Expect(kafkaTopic.Namespace).To(Equal(namespace))
			Expect(kafkaTopic.GetLabels()).To(Equal(map[string]string{"strimzi.io/cluster": "kafka"}))
			Expect(kafkaTopic.Spec.Partitions).To(Equal(&kafkaTopicPartitions))
			Expect(kafkaTopic.Spec.Replicas).To(Equal(&kafkaTopicReplicas))
			Expect(kafkaTopic.Spec.TopicName).To(Equal(&kafkaTopicName))

			topicConfigFile, err := os.ReadFile("./test/data/kafka/kafka_topic_config.json")
			checkError(err)
			expectedKafkaTopicConfig := bytes.NewBuffer([]byte{})
			err = json.Compact(expectedKafkaTopicConfig, topicConfigFile)
			checkError(err)

			actualKafkaTopicConfig := bytes.NewBuffer([]byte{})
			err = json.Compact(actualKafkaTopicConfig, kafkaTopic.Spec.Config.Raw)
			checkError(err)

			Expect(actualKafkaTopicConfig).To(Equal(expectedKafkaTopicConfig))
		})
	})
})
