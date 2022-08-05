package controllers

import (
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("XJoinDataSource", func() {
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
		It("Should create a XJoinDataSourcePipeline", func() {
			reconciler := DatasourceTestReconciler{
				Namespace: namespace,
				Name:      "test-data-source",
				K8sClient: k8sClient,
			}
			createdDataSource := reconciler.ReconcileNew()
		
			dataSourcePipelineName := createdDataSource.Name + "." + createdDataSource.Status.RefreshingVersion
			datasourcePipelineKey := types.NamespacedName{Name: dataSourcePipelineName, Namespace: namespace}
			createdDatasourcePipeline := &v1alpha1.XJoinDataSourcePipeline{}
			k8sGet(datasourcePipelineKey, createdDatasourcePipeline)

			Expect(createdDatasourcePipeline.Name).To(Equal(dataSourcePipelineName))
			Expect(createdDatasourcePipeline.Spec.Name).To(Equal(createdDataSource.Name))
			Expect(createdDatasourcePipeline.Spec.Version).To(Equal(createdDataSource.Status.RefreshingVersion))
			Expect(createdDatasourcePipeline.Spec.AvroSchema).To(Equal(createdDataSource.Spec.AvroSchema))
			Expect(createdDatasourcePipeline.Spec.DatabaseHostname).To(Equal(createdDataSource.Spec.DatabaseHostname))
			Expect(createdDatasourcePipeline.Spec.DatabasePort).To(Equal(createdDataSource.Spec.DatabasePort))
			Expect(createdDatasourcePipeline.Spec.DatabaseUsername).To(Equal(createdDataSource.Spec.DatabaseUsername))
			Expect(createdDatasourcePipeline.Spec.DatabasePassword).To(Equal(createdDataSource.Spec.DatabasePassword))
			Expect(createdDatasourcePipeline.Spec.DatabaseName).To(Equal(createdDataSource.Spec.DatabaseName))
			Expect(createdDatasourcePipeline.Spec.DatabaseTable).To(Equal(createdDataSource.Spec.DatabaseTable))
			Expect(createdDatasourcePipeline.Spec.Pause).To(Equal(createdDataSource.Spec.Pause))

			controller := true
			blockOwnerDeletion := true
			dataSourceOwnerReference := metav1.OwnerReference{
				APIVersion:         "v1alpha1",
				Kind:               "XJoinDataSource",
				Name:               createdDataSource.Name,
				UID:                createdDataSource.UID,
				Controller:         &controller,
				BlockOwnerDeletion: &blockOwnerDeletion,
			}
			Expect(createdDatasourcePipeline.OwnerReferences).To(HaveLen(1))
			Expect(createdDatasourcePipeline.OwnerReferences).To(ContainElement(dataSourceOwnerReference))
		})
	})
})
