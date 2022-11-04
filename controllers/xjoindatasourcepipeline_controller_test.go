package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/RedHatInsights/strimzi-client-go/apis/kafka.strimzi.io/v1beta2"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("XJoinDataSourcePipeline", func() {
	var namespace string

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
			reconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      "test-data-source-pipeline",
				K8sClient: k8sClient,
			}
			createdDataSourcePipeline := reconciler.ReconcileNew()
			Expect(createdDataSourcePipeline.Finalizers).To(HaveLen(1))
			Expect(createdDataSourcePipeline.Finalizers).To(ContainElement("finalizer.xjoin.datasourcepipeline.cloud.redhat.com"))
		})

		It("Creates a Debezium Kafka Connector", func() {
			reconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      "test-data-source-pipeline",
				K8sClient: k8sClient,
			}
			reconciler.ReconcileNew()

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
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

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
			reconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      "test-data-source-pipeline",
				K8sClient: k8sClient,
			}
			reconciler.ReconcileNew()

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
			reconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      "test-data-source-pipeline",
				K8sClient: k8sClient,
			}
			reconciler.ReconcileNew()

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
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

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

	Context("Reconcile Deletion", func() {
		It("Deletes the Debezium Kafka Connector", func() {
			name := "test-data-source-pipeline"
			reconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      name,
				K8sClient: k8sClient,
			}
			createdDataSourcePipeline := reconciler.ReconcileNew()

			connectors := &v1beta2.KafkaConnectorList{}
			err := k8sClient.List(context.Background(), connectors, client.InNamespace(namespace))
			checkError(err)
			Expect(connectors.Items).To(HaveLen(1))

			err = k8sClient.Delete(context.Background(), &createdDataSourcePipeline)
			checkError(err)
			reconciler.ReconcileDelete()

			info := httpmock.GetCallCountInfo()
			count := info["GET http://connect-connect-api."+namespace+".svc:8083/connectors/xjoindatasourcepipeline."+name+".1234"]
			Expect(count).To(Equal(6))

			connectors = &v1beta2.KafkaConnectorList{}
			err = k8sClient.List(context.Background(), connectors, client.InNamespace(namespace))
			checkError(err)
			Expect(connectors.Items).To(HaveLen(0))
		})

		It("Deletes the Avro Schema", func() {
			name := "test-data-source-pipeline"
			reconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      name,
				K8sClient: k8sClient,
			}
			createdDataSourcePipeline := reconciler.ReconcileNew()

			err := k8sClient.Delete(context.Background(), &createdDataSourcePipeline)
			checkError(err)
			reconciler.ReconcileDelete()

			info := httpmock.GetCallCountInfo()
			count := info["DELETE http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+name+".1234-value"]
			Expect(count).To(Equal(1))
		})

		It("Deletes the Kafka Topic", func() {
			name := "test-data-source-pipeline"
			reconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      name,
				K8sClient: k8sClient,
			}
			createdDataSourcePipeline := reconciler.ReconcileNew()

			topics := &v1beta2.KafkaTopicList{}
			err := k8sClient.List(context.Background(), topics, client.InNamespace(namespace))
			checkError(err)
			Expect(topics.Items).To(HaveLen(1))

			err = k8sClient.Delete(context.Background(), &createdDataSourcePipeline)
			checkError(err)
			reconciler.ReconcileDelete()

			topics = &v1beta2.KafkaTopicList{}
			err = k8sClient.List(context.Background(), topics, client.InNamespace(namespace))
			checkError(err)
			Expect(topics.Items).To(HaveLen(0))
		})
	})
})
