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
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	v1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("XJoinIndexPipeline", func() {
	var namespace string

	var newXJoinIndexPipelineReconciler = func() *XJoinIndexPipelineReconciler {
		return NewXJoinIndexPipelineReconciler(
			k8sClient,
			scheme.Scheme,
			testLogger,
			record.NewFakeRecorder(10),
			namespace,
			true)
	}

	var createValidIndexPipeline = func(name string, configFileName string, customSubgraphImages ...v1alpha1.CustomSubgraphImage) {
		ctx := context.Background()
		indexAvroSchema, err := os.ReadFile("./test/data/avro/" + configFileName + ".json")
		checkError(err)
		xjoinIndexName := "test-xjoin-index"

		//XjoinIndexPipeline requires an XJoinIndex owner. Create one here
		indexSpec := v1alpha1.XJoinIndexSpec{
			AvroSchema:           string(indexAvroSchema),
			Pause:                false,
			CustomSubgraphImages: customSubgraphImages,
		}

		index := &v1alpha1.XJoinIndex{
			ObjectMeta: metav1.ObjectMeta{
				Name:      xjoinIndexName,
				Namespace: namespace,
			},
			Spec: indexSpec,
			TypeMeta: metav1.TypeMeta{
				APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
				Kind:       "XJoinIndex",
			},
		}

		Expect(k8sClient.Create(ctx, index)).Should(Succeed())

		//create the XJoinIndexPipeline
		indexPipelineSpec := v1alpha1.XJoinIndexPipelineSpec{
			Name:                 name,
			Version:              "1234",
			AvroSchema:           string(indexAvroSchema),
			Pause:                false,
			CustomSubgraphImages: customSubgraphImages,
		}

		blockOwnerDeletion := true
		controller := true
		indexPipeline := &v1alpha1.XJoinIndexPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         common.IndexGVK.Version,
						Kind:               common.IndexGVK.Kind,
						Name:               xjoinIndexName,
						Controller:         &controller,
						BlockOwnerDeletion: &blockOwnerDeletion,
						UID:                "a6778b9b-dfed-4d41-af53-5ebbcddb7535",
					},
				},
			},
			Spec: indexPipelineSpec,
			TypeMeta: metav1.TypeMeta{
				APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
				Kind:       "XJoinIndexPipeline",
			},
		}

		Expect(k8sClient.Create(ctx, indexPipeline)).Should(Succeed())

		//validate indexPipeline spec is created correctly
		indexPipelineLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
		createdIndexPipeline := &v1alpha1.XJoinIndexPipeline{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, indexPipelineLookupKey, createdIndexPipeline)
			if err != nil {
				return false
			}
			return true
		}, k8sGetTimeout, k8sGetInterval).Should(BeTrue())

		Expect(createdIndexPipeline.Spec.Name).Should(Equal(name))
		Expect(createdIndexPipeline.Spec.Version).Should(Equal("1234"))
		Expect(createdIndexPipeline.Spec.Pause).Should(Equal(false))
		Expect(createdIndexPipeline.Spec.AvroSchema).Should(Equal(string(indexAvroSchema)))
		Expect(createdIndexPipeline.Spec.CustomSubgraphImages).Should(Equal(customSubgraphImages))
	}

	var reconcileIndexPipeline = func(name string) v1alpha1.XJoinIndexPipeline {
		//elasticsearch index mocks
		httpmock.RegisterResponder(
			"HEAD",
			"http://localhost:9200/xjoinindexpipeline.test-index-pipeline.1234",
			httpmock.NewStringResponder(404, `{}`))

		httpmock.RegisterResponder(
			"PUT",
			"http://localhost:9200/xjoinindexpipeline.test-index-pipeline.1234",
			httpmock.NewStringResponder(201, `{}`))

		//avro schema mocks
		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline.test-index-pipeline.1234-value/versions/1",
			httpmock.NewStringResponder(404, `{"message":"No version '1' found for artifact with ID 'xjoinindexpipelinepipeline.test-index-pipeline.1234-value' in group 'null'.","error_code":40402}`))

		httpmock.RegisterResponder(
			"POST",
			"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline.test-index-pipeline.1234-value/versions",
			httpmock.NewStringResponder(200, `{"createdBy":"","createdOn":"2022-07-27T17:28:11+0000","modifiedBy":"","modifiedOn":"2022-07-27T17:28:11+0000","id":1,"version":1,"type":"AVRO","globalId":1,"state":"ENABLED","groupId":"null","contentId":1,"references":[]}`))

		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline.test-index-pipeline.1234-value/versions/latest",
			httpmock.NewStringResponder(200, `{"schema":"{\"name\":\"Value\",\"namespace\":\"xjoinindexpipelinepipeline.test-index-pipeline\"}","schemaType":"AVRO","references":[]}`))

		//graphql schema mocks
		httpmock.RegisterResponder(
			"GET",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline.1234/versions",
			httpmock.NewStringResponder(404, `{}`))

		httpmock.RegisterResponder(
			"POST",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts",
			httpmock.NewStringResponder(201, `{}`))

		httpmock.RegisterResponder(
			"PUT",
			"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline.1234/meta",
			httpmock.NewStringResponder(200, `{}`))

		ctx := context.Background()

		xjoinIndexPipelineReconciler := newXJoinIndexPipelineReconciler()

		indexLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
		createdIndexPipeline := &v1alpha1.XJoinIndexPipeline{}

		result, err := xjoinIndexPipelineReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: indexLookupKey})
		checkError(err)
		Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

		Eventually(func() bool {
			err := k8sClient.Get(ctx, indexLookupKey, createdIndexPipeline)
			if err != nil {
				return false
			}
			return true
		}, k8sGetTimeout, k8sGetInterval).Should(BeTrue())

		return *createdIndexPipeline
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
		It("Should add a finalizer to the indexPipeline", func() {
			indexPipelineName := "test-index-pipeline"
			createValidIndexPipeline(indexPipelineName, "xjoinindex")
			createdIndexPipeline := reconcileIndexPipeline(indexPipelineName)
			Expect(createdIndexPipeline.Finalizers).To(HaveLen(1))
			Expect(createdIndexPipeline.Finalizers).To(ContainElement("finalizer.xjoin.indexpipeline.cloud.redhat.com"))
		})

		It("Should create an Elasticsearch Index", func() {
			indexPipelineName := "test-index-pipeline"
			createValidIndexPipeline(indexPipelineName, "xjoinindex")
			reconcileIndexPipeline(indexPipelineName)

			info := httpmock.GetCallCountInfo()
			count := info["PUT http://localhost:9200/xjoinindexpipeline.test-index-pipeline.1234"]
			Expect(count).To(Equal(1))
		})

		It("Should create an Elasticsearch Connector", func() {
			indexPipelineName := "test-index-pipeline"
			createValidIndexPipeline(indexPipelineName, "xjoinindex")
			reconcileIndexPipeline(indexPipelineName)

			connectorName := "xjoinindexpipeline.test-index-pipeline.1234"
			connectorLookupKey := types.NamespacedName{Name: connectorName, Namespace: namespace}
			elasticsearchConnector := &v1beta2.KafkaConnector{}

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), connectorLookupKey, elasticsearchConnector)
				if err != nil {
					return false
				}
				return true
			}, k8sGetTimeout, k8sGetInterval).Should(BeTrue())

			elasticsearchClass := "io.confluent.connect.elasticsearch.ElasticsearchSinkConnector"
			connectorPause := false

			Expect(elasticsearchConnector.Name).To(Equal(connectorName))
			Expect(elasticsearchConnector.Namespace).To(Equal(namespace))
			Expect(elasticsearchConnector.Spec.Class).To(Equal(&elasticsearchClass))
			Expect(elasticsearchConnector.Spec.Pause).To(Equal(&connectorPause))
			Expect(elasticsearchConnector.GetLabels()).To(Equal(map[string]string{"strimzi.io/cluster": "connect"}))

			//config comparison
			expectedElasticsearchConfig := LoadExpectedKafkaResourceConfig("./test/data/kafka/elasticsearch_config.json")
			actualElasticsearchConfig := bytes.NewBuffer([]byte{})
			err := json.Compact(actualElasticsearchConfig, elasticsearchConnector.Spec.Config.Raw)
			checkError(err)
			Expect(actualElasticsearchConfig).To(Equal(expectedElasticsearchConfig))
		})

		It("Should create a Kafka Topic", func() {
			indexPipelineName := "test-index-pipeline"
			createValidIndexPipeline(indexPipelineName, "xjoinindex")
			reconcileIndexPipeline(indexPipelineName)

			topicName := "xjoinindexpipeline.test-index-pipeline.1234"
			topicLookupKey := types.NamespacedName{Name: topicName, Namespace: namespace}
			topic := &v1beta2.KafkaTopic{}

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), topicLookupKey, topic)
				if err != nil {
					return false
				}
				return true
			}, k8sGetTimeout, k8sGetInterval).Should(BeTrue())

			topicReplicas := int32(1)
			topicPartitions := int32(1)

			Expect(topic.Name).To(Equal(topicName))
			Expect(topic.Namespace).To(Equal(namespace))
			Expect(topic.Spec.Replicas).To(Equal(&topicReplicas))
			Expect(topic.Spec.Partitions).To(Equal(&topicPartitions))

			//config comparison
			expectedTopicConfig := LoadExpectedKafkaResourceConfig("./test/data/kafka/topic_config.json")
			actualTopicConfig := bytes.NewBuffer([]byte{})
			err := json.Compact(actualTopicConfig, topic.Spec.Config.Raw)
			checkError(err)
			Expect(actualTopicConfig).To(Equal(expectedTopicConfig))
		})

		It("Should create an Avro Schema", func() {
			indexPipelineName := "test-index-pipeline"
			createValidIndexPipeline(indexPipelineName, "xjoinindex")
			reconcileIndexPipeline(indexPipelineName)

			//TODO validate the body of the request is correct
			//validates the correct API calls were made
			info := httpmock.GetCallCountInfo()
			count := info["GET http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline.test-index-pipeline.1234-value/versions/1"]
			Expect(count).To(Equal(1))

			count = info["POST http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline.test-index-pipeline.1234-value/versions"]
			Expect(count).To(Equal(1))

			count = info["GET http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline.test-index-pipeline.1234-value/versions/latest"]
			Expect(count).To(Equal(1))
		})

		It("Should create a GraphQL Schema", func() {
			indexPipelineName := "test-index-pipeline"
			createValidIndexPipeline(indexPipelineName, "xjoinindex")
			reconcileIndexPipeline(indexPipelineName)

			//TODO validate the body of the request is correct
			//validates the correct API calls were made
			info := httpmock.GetCallCountInfo()
			count := info["GET http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline.1234/versions"]
			Expect(count).To(Equal(1))

			count = info["POST http://apicurio:1080/apis/registry/v2/groups/default/artifacts"]
			Expect(count).To(Equal(1))

			count = info["PUT http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline.1234/meta"]
			Expect(count).To(Equal(1))
		})

		It("Should create an xjoin-core deployment", func() {
			indexPipelineName := "test-index-pipeline"
			createValidIndexPipeline(indexPipelineName, "xjoinindex")
			reconcileIndexPipeline(indexPipelineName)

			deploymentName := "xjoin-core-xjoinindexpipeline-test-index-pipeline-1234"
			deploymentLookupKey := types.NamespacedName{Name: deploymentName, Namespace: namespace}
			deployment := &v1.Deployment{}

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), deploymentLookupKey, deployment)
				if err != nil {
					return false
				}
				return true
			}, k8sGetTimeout, k8sGetInterval).Should(BeTrue())

			replicas := int32(1)
			revisionHistoryLimit := int32(10)
			progressDeadlineSeconds := int32(600)

			Expect(deployment.Name).To(Equal(deploymentName))
			Expect(deployment.Namespace).To(Equal(namespace))
			Expect(deployment.Spec.Replicas).To(Equal(&replicas))
			Expect(deployment.Spec.Selector.MatchLabels).To(Equal(map[string]string{
				"app":         "xjoin-core-xjoinindexpipeline-test-index-pipeline-1234",
				"xjoin.index": "xjoin-core-xjoinindexpipeline-test-index-pipeline",
			}))
			Expect(deployment.GetLabels()).To(Equal(map[string]string{
				"app":         "xjoin-core-xjoinindexpipeline-test-index-pipeline-1234",
				"xjoin.index": "xjoin-core-xjoinindexpipeline-test-index-pipeline",
			}))
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("xjoin-core-xjoinindexpipeline-test-index-pipeline-1234"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("quay.io/ckyrouac/xjoin-core:latest"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(5))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElements([]v12.EnvVar{
				{
					Name:      "SOURCE_TOPICS",
					Value:     "", //TODO
					ValueFrom: nil,
				},
				{
					Name:      "SINK_TOPIC",
					Value:     "xjoinindexpipeline.test",
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_URL",
					Value:     "http://apicurio:1080/apis/registry/v2",
					ValueFrom: nil,
				},
				{
					Name:      "KAFKA_BOOTSTRAP",
					Value:     "localhost:9092",
					ValueFrom: nil,
				},
				{
					Name:      "SINK_SCHEMA",
					Value:     `{"type":"record","name":"Value","namespace":"test-index-pipeline"}`,
					ValueFrom: nil,
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(BeNil())
			Expect(deployment.Spec.Strategy.Type).To(Equal(v1.DeploymentStrategyType("RollingUpdate")))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.MinReadySeconds).To(Equal(int32(0)))
			Expect(deployment.Spec.RevisionHistoryLimit).To(Equal(&revisionHistoryLimit))
			Expect(deployment.Spec.Paused).To(Equal(false))
			Expect(deployment.Spec.ProgressDeadlineSeconds).To(Equal(&progressDeadlineSeconds))
		})

		It("Should create an xjoin-api-subgraph deployment", func() {
			indexPipelineName := "test-index-pipeline"
			createValidIndexPipeline(indexPipelineName, "xjoinindex")
			reconcileIndexPipeline(indexPipelineName)

			deploymentName := "xjoinindexpipeline-test-index-pipeline-1234"
			deploymentLookupKey := types.NamespacedName{Name: deploymentName, Namespace: namespace}
			deployment := &v1.Deployment{}

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), deploymentLookupKey, deployment)
				if err != nil {
					return false
				}
				return true
			}, k8sGetTimeout, k8sGetInterval).Should(BeTrue())

			replicas := int32(1)
			revisionHistoryLimit := int32(10)
			progressDeadlineSeconds := int32(600)

			Expect(deployment.Name).To(Equal(deploymentName))
			Expect(deployment.Namespace).To(Equal(namespace))
			Expect(deployment.Spec.Replicas).To(Equal(&replicas))
			Expect(deployment.GetLabels()).To(Equal(map[string]string{
				"app":         "xjoinindexpipeline-test-index-pipeline-1234",
				"xjoin.index": "xjoinindexpipeline-test-index-pipeline",
			}))
			Expect(deployment.Spec.Selector.MatchLabels).To(Equal(map[string]string{
				"app":         "xjoinindexpipeline-test-index-pipeline-1234",
				"xjoin.index": "xjoinindexpipeline-test-index-pipeline",
			}))

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("xjoinindexpipeline-test-index-pipeline-1234"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("quay.io/ckyrouac/xjoin-api-subgraph:latest"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(9))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElements([]v12.EnvVar{
				{
					Name:      "AVRO_SCHEMA",
					Value:     `{"type":"record","name":"Value","namespace":"test-index-pipeline"}`,
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_PROTOCOL",
					Value:     "http",
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_HOSTNAME",
					Value:     "apicurio.test.svc",
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_PORT",
					Value:     "1080",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_URL",
					Value:     "http://localhost:9200",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_USERNAME",
					Value:     "xjoin",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_PASSWORD",
					Value:     "xjoin1337",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_INDEX",
					Value:     "xjoinindexpipeline.test-index-pipeline.1234",
					ValueFrom: nil,
				},
				{
					Name:      "GRAPHQL_SCHEMA_NAME",
					Value:     "xjoinindexpipeline.test-index-pipeline.1234",
					ValueFrom: nil,
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(Equal([]v12.ContainerPort{{
				Name:          "web",
				HostPort:      int32(0),
				ContainerPort: int32(8000),
				Protocol:      "TCP",
				HostIP:        "",
			}}))
			Expect(deployment.Spec.Strategy.Type).To(Equal(v1.DeploymentStrategyType("RollingUpdate")))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.MinReadySeconds).To(Equal(int32(0)))
			Expect(deployment.Spec.RevisionHistoryLimit).To(Equal(&revisionHistoryLimit))
			Expect(deployment.Spec.Paused).To(Equal(false))
			Expect(deployment.Spec.ProgressDeadlineSeconds).To(Equal(&progressDeadlineSeconds))
		})

		It("Should create custom subgraph graphql schema", func() {
			//graphql schema mocks
			httpmock.RegisterResponder(
				"GET",
				"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline-test-custom-image.1234/versions",
				httpmock.NewStringResponder(404, `{}`))

			httpmock.RegisterResponder(
				"POST",
				"http://apicurio:1080/apis/registry/v2/groups/default/artifacts",
				httpmock.NewStringResponder(201, `{}`))

			httpmock.RegisterResponder(
				"PUT",
				"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline-test-custom-image.1234/meta",
				httpmock.NewStringResponder(200, `{}`))

			indexPipelineName := "test-index-pipeline"
			createValidIndexPipeline(indexPipelineName, "xjoinindex", v1alpha1.CustomSubgraphImage{
				Name:  "test-custom-image",
				Image: "quay.io/ckyrouac/host-inventory-subgraph:latest",
			})
			reconcileIndexPipeline(indexPipelineName)

			//TODO validate the body of the request is correct
			//validates the correct API calls were made
			info := httpmock.GetCallCountInfo()
			count := info["GET http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline.1234/versions"]
			Expect(count).To(Equal(1))

			count = info["POST http://apicurio:1080/apis/registry/v2/groups/default/artifacts"]
			Expect(count).To(Equal(2)) //called once for generic gql schema, then a second time for custom subgraph schema

			count = info["PUT http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline.1234/meta"]
			Expect(count).To(Equal(1))
		})

		It("Should create custom subgraph deployments", func() {
			//graphql schema mocks
			httpmock.RegisterResponder(
				"GET",
				"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline-test-custom-image.1234/versions",
				httpmock.NewStringResponder(404, `{}`))

			httpmock.RegisterResponder(
				"POST",
				"http://apicurio:1080/apis/registry/v2/groups/default/artifacts",
				httpmock.NewStringResponder(201, `{}`))

			httpmock.RegisterResponder(
				"PUT",
				"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoinindexpipeline.test-index-pipeline-test-custom-image.1234/meta",
				httpmock.NewStringResponder(200, `{}`))

			indexPipelineName := "test-index-pipeline"
			createValidIndexPipeline(indexPipelineName, "xjoinindex", v1alpha1.CustomSubgraphImage{
				Name:  "test-custom-image",
				Image: "quay.io/ckyrouac/host-inventory-subgraph:latest",
			})
			reconcileIndexPipeline(indexPipelineName)

			deploymentName := "xjoinindexpipeline-test-index-pipeline-test-custom-image-1234"
			deploymentLookupKey := types.NamespacedName{Name: deploymentName, Namespace: namespace}
			deployment := &v1.Deployment{}

			//test

			deployments := &v1.DeploymentList{}
			err := k8sClient.List(context.Background(), deployments)
			checkError(err)
			//test

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), deploymentLookupKey, deployment)
				if err != nil {
					return false
				}
				return true
			}, k8sGetTimeout, k8sGetInterval).Should(BeTrue())

			replicas := int32(1)
			revisionHistoryLimit := int32(10)
			progressDeadlineSeconds := int32(600)

			Expect(deployment.Name).To(Equal(deploymentName))
			Expect(deployment.Namespace).To(Equal(namespace))
			Expect(deployment.Spec.Replicas).To(Equal(&replicas))
			Expect(deployment.GetLabels()).To(Equal(map[string]string{
				"app":         "xjoinindexpipeline-test-index-pipeline-test-custom-image-1234",
				"xjoin.index": "xjoinindexpipeline-test-index-pipeline-test-custom-image",
			}))
			Expect(deployment.Spec.Selector.MatchLabels).To(Equal(map[string]string{
				"app":         "xjoinindexpipeline-test-index-pipeline-test-custom-image-1234",
				"xjoin.index": "xjoinindexpipeline-test-index-pipeline-test-custom-image",
			}))

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal(deploymentName))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("quay.io/ckyrouac/host-inventory-subgraph:latest"))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(HaveLen(9))
			Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElements([]v12.EnvVar{
				{
					Name:      "AVRO_SCHEMA",
					Value:     `{"type":"record","name":"Value","namespace":"test-index-pipeline"}`,
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_PROTOCOL",
					Value:     "http",
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_HOSTNAME",
					Value:     "apicurio.test.svc",
					ValueFrom: nil,
				},
				{
					Name:      "SCHEMA_REGISTRY_PORT",
					Value:     "1080",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_URL",
					Value:     "http://localhost:9200",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_USERNAME",
					Value:     "xjoin",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_PASSWORD",
					Value:     "xjoin1337",
					ValueFrom: nil,
				},
				{
					Name:      "ELASTIC_SEARCH_INDEX",
					Value:     "xjoinindexpipeline.test-index-pipeline.1234",
					ValueFrom: nil,
				},
				{
					Name:      "GRAPHQL_SCHEMA_NAME",
					Value:     "xjoinindexpipeline.test-index-pipeline-test-custom-image.1234",
					ValueFrom: nil,
				},
			}))
			Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(BeNil())
			Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(Equal([]v12.ContainerPort{{
				Name:          "web",
				HostPort:      int32(0),
				ContainerPort: int32(8000),
				Protocol:      "TCP",
				HostIP:        "",
			}}))
			Expect(deployment.Spec.Strategy.Type).To(Equal(v1.DeploymentStrategyType("RollingUpdate")))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxUnavailable.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.Type).To(Equal(intstr.Type(1)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.IntVal).To(Equal(int32(0)))
			Expect(deployment.Spec.Strategy.RollingUpdate.MaxSurge.StrVal).To(Equal("25%"))
			Expect(deployment.Spec.MinReadySeconds).To(Equal(int32(0)))
			Expect(deployment.Spec.RevisionHistoryLimit).To(Equal(&revisionHistoryLimit))
			Expect(deployment.Spec.Paused).To(Equal(false))
			Expect(deployment.Spec.ProgressDeadlineSeconds).To(Equal(&progressDeadlineSeconds))
		})

		It("Should create an Elasticsearch Pipeline when the AvroSchema contains at least one JSON field", func() {
			dataSourceName := "testdatasource"
			createValidDataSource(dataSourceName, namespace)
			createdDataSource := reconcileDataSourceToBeValid(dataSourceName, namespace)

			response, err := os.ReadFile("./test/data/apicurio/datasource-latest-version.json")
			checkError(err)
			httpmock.RegisterResponder(
				"GET",
				"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline.testdatasource."+createdDataSource.Status.ActiveVersion+"-value/versions/latest",
				httpmock.NewStringResponder(200, string(response)))

			httpmock.RegisterResponder(
				"GET",
				"http://localhost:9200/_ingest/pipeline/xjoinindexpipeline.test-index-pipeline.1234",
				httpmock.NewStringResponder(404, "{}").Once())

			httpmock.RegisterResponder(
				"PUT",
				"http://localhost:9200/_ingest/pipeline/xjoinindexpipeline.test-index-pipeline.1234",
				httpmock.NewStringResponder(200, "{}"))

			indexPipelineName := "test-index-pipeline"
			createValidIndexPipeline(indexPipelineName, "xjoinindex-with-json-field")
			reconcileIndexPipeline(indexPipelineName)

			info := httpmock.GetCallCountInfo()
			count := info["GET http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline.testdatasource."+createdDataSource.Status.ActiveVersion+"-value/versions/latest"]
			Expect(count).To(Equal(1))

			count = info["GET http://localhost:9200/_ingest/pipeline/xjoinindexpipeline.test-index-pipeline.1234"]
			Expect(count).To(Equal(1))

			count = info["PUT http://localhost:9200/_ingest/pipeline/xjoinindexpipeline.test-index-pipeline.1234"]
			Expect(count).To(Equal(1))
		})
	})
})
