package controllers

import (
	"context"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("XJoinIndex", func() {
	var namespace string

	var newXJoinIndexReconciler = func() *XJoinIndexReconciler {
		return NewXJoinIndexReconciler(
			k8sClient,
			scheme.Scheme,
			testLogger,
			record.NewFakeRecorder(10),
			namespace,
			true)
	}

	var createValidIndex = func(name string) {
		ctx := context.Background()

		httpmock.RegisterResponder(
			"GET",
			"http://example-apicurioregistry-kafkasql-service.test.svc:1080/apis/ccompat/v6/subjects",
			httpmock.NewStringResponder(200, `[]`))

		responder, err := httpmock.NewJsonResponder(200, httpmock.File("./test/apicurio/response.json"))
		checkError(err)
		httpmock.RegisterResponder(
			"GET",
			"http://example-apicurioregistry-kafkasql-service.test.svc:8080/apis/registry/v2/search/artifacts?limit=500&labels=graphql",
			responder)

		httpmock.RegisterResponder(
			"GET",
			"http://localhost:9200/_ingest/pipeline/xjoinindex.test-index%2A",
			httpmock.NewStringResponder(404, "{}"))

		httpmock.RegisterResponder(
			"GET",
			"http://localhost:9200/_cat/indices/xjoinindex.test-index.%2A?format=JSON&h=index",
			httpmock.NewStringResponder(200, "[]"))

		customSubgraphImage := []v1alpha1.CustomSubgraphImage{{
			Name:  "test",
			Image: "test",
		}}

		indexSpec := v1alpha1.XJoinIndexSpec{
			AvroSchema:           "{}",
			Pause:                false,
			CustomSubgraphImages: customSubgraphImage,
		}

		index := &v1alpha1.XJoinIndex{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: indexSpec,
			TypeMeta: metav1.TypeMeta{
				APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
				Kind:       "XJoinIndex",
			},
		}

		Expect(k8sClient.Create(ctx, index)).Should(Succeed())

		//validate index spec is created correctly
		indexLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
		createdIndex := &v1alpha1.XJoinIndex{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, indexLookupKey, createdIndex)
			if err != nil {
				return false
			}
			return true
		}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
		Expect(createdIndex.Spec.Pause).Should(Equal(false))
		Expect(createdIndex.Spec.AvroSchema).Should(Equal("{}"))
		Expect(createdIndex.Spec.CustomSubgraphImages).Should(Equal(customSubgraphImage))
	}

	var reconcileIndex = func(name string) v1alpha1.XJoinIndex {
		ctx := context.Background()

		xjoinIndexReconciler := newXJoinIndexReconciler()

		indexLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
		createdIndex := &v1alpha1.XJoinIndex{}

		result, err := xjoinIndexReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: indexLookupKey})
		checkError(err)
		Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

		Eventually(func() bool {
			err := k8sClient.Get(ctx, indexLookupKey, createdIndex)
			if err != nil {
				return false
			}
			return true
		}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
		Expect(createdIndex.Status.ActiveVersion).To(Equal(""))
		Expect(createdIndex.Status.ActiveVersionIsValid).To(Equal(false))
		Expect(createdIndex.Status.RefreshingVersion).ToNot(Equal(""))
		Expect(createdIndex.Status.RefreshingVersionIsValid).To(Equal(false))
		Expect(createdIndex.Status.SpecHash).ToNot(Equal(""))
		Expect(createdIndex.Finalizers).To(HaveLen(1))
		Expect(createdIndex.Finalizers).To(ContainElement("finalizer.xjoin.index.cloud.redhat.com"))

		info := httpmock.GetCallCountInfo()
		count := info["GET http://example-apicurioregistry-kafkasql-service.test.svc:1080/apis/ccompat/v6/subjects"]
		Expect(count).To(Equal(1))

		return *createdIndex
	}

	var createElasticsearchSecret = func() {
		ctx := context.Background()
		secret := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "xjoin-elasticsearch",
				Namespace: namespace,
			},
			Type: "opaque",
			StringData: map[string]string{
				"endpoint": "http://localhost:9200",
				"password": "xjoin1337",
				"username": "xjoin",
			},
		}
		err := k8sClient.Create(ctx, &secret)
		checkError(err)
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
		It("Should create a XJoinIndexPipeline", func() {
			indexName := "test-index"
			createElasticsearchSecret()
			createValidIndex(indexName)
			createdIndex := reconcileIndex(indexName)
			indexPipelineName := createdIndex.Name + "." + createdIndex.Status.RefreshingVersion

			indexPipelineKey := types.NamespacedName{Name: indexPipelineName, Namespace: namespace}
			createdIndexPipeline := &v1alpha1.XJoinIndexPipeline{}
			k8sGet(indexPipelineKey, createdIndexPipeline)
			Expect(createdIndexPipeline.Name).To(Equal(indexPipelineName))
			Expect(createdIndexPipeline.Spec.Name).To(Equal(createdIndex.Name))
			Expect(createdIndexPipeline.Spec.Version).To(Equal(createdIndex.Status.RefreshingVersion))
			Expect(createdIndexPipeline.Spec.AvroSchema).To(Equal(createdIndex.Spec.AvroSchema))
			Expect(createdIndexPipeline.Spec.Pause).To(Equal(createdIndex.Spec.Pause))
			Expect(createdIndexPipeline.Spec.CustomSubgraphImages).To(Equal(createdIndex.Spec.CustomSubgraphImages))

			controller := true
			blockOwnerDeletion := true
			indexOwnerReference := metav1.OwnerReference{
				APIVersion:         "v1alpha1",
				Kind:               "XJoinIndex",
				Name:               createdIndex.Name,
				UID:                createdIndex.UID,
				Controller:         &controller,
				BlockOwnerDeletion: &blockOwnerDeletion,
			}
			Expect(createdIndexPipeline.OwnerReferences).To(HaveLen(1))
			Expect(createdIndexPipeline.OwnerReferences).To(ContainElement(indexOwnerReference))
		})
	})
})
