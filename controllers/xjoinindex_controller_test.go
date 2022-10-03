package controllers

import (
	"context"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("XJoinIndex", func() {
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
		It("Should create a XJoinIndexPipeline", func() {
			reconciler := IndexTestReconciler{
				Namespace: namespace,
				Name:      "test-index",
				K8sClient: k8sClient,
			}
			createdIndex := reconciler.ReconcileNew()

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

	Context("Reconcile Delete", func() {
		It("Should delete a XJoinIndexPipeline", func() {
			reconciler := IndexTestReconciler{
				Namespace: namespace,
				Name:      "test-index",
				K8sClient: k8sClient,
			}
			createdIndex := reconciler.ReconcileNew()

			indexPipelineList := &v1alpha1.XJoinIndexPipelineList{}
			err := k8sClient.List(context.Background(), indexPipelineList)
			checkError(err)
			Expect(indexPipelineList.Items).To(HaveLen(1))

			err = k8sClient.Delete(context.Background(), &createdIndex)
			checkError(err)
			reconciler.ReconcileDelete()

			err = k8sClient.List(context.Background(), indexPipelineList)
			checkError(err)
			Expect(indexPipelineList.Items).To(HaveLen(0))

		})
	})
})
