package controllers_test

import (
	"context"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
			err := k8sClient.List(context.Background(), indexPipelineList, client.InNamespace(namespace))
			checkError(err)
			Expect(indexPipelineList.Items).To(HaveLen(1))

			err = k8sClient.Delete(context.Background(), &createdIndex)
			checkError(err)
			reconciler.ReconcileDelete()

			err = k8sClient.List(context.Background(), indexPipelineList, client.InNamespace(namespace))
			checkError(err)
			Expect(indexPipelineList.Items).To(HaveLen(0))

		})
	})

	Context("Pipeline management", func() {
		It("Should replace the active pipeline with the refreshing IndexPipeline when the refreshing pipeline becomes valid", func() {
			//setup initial state with an invalid refreshing pipeline
			indexReconciler := IndexTestReconciler{
				Namespace:          namespace,
				Name:               "test-index",
				AvroSchemaFileName: "xjoinindex-with-referenced-field",
				K8sClient:          k8sClient,
			}
			createdIndex := indexReconciler.ReconcileNew()

			Expect(createdIndex.Status.RefreshingVersion).ToNot(Equal(""))
			Expect(createdIndex.Status.RefreshingVersionIsValid).To(Equal(false))
			Expect(createdIndex.Status.ActiveVersion).To(Equal(""))
			Expect(createdIndex.Status.ActiveVersionIsValid).To(Equal(false))

			//create a valid datasource
			dataSourceName := "testdatasource"
			datasourceReconciler := DatasourceTestReconciler{
				Namespace: namespace,
				Name:      dataSourceName,
				K8sClient: k8sClient,
			}
			datasourceReconciler.ReconcileNew()
			createdDataSource := datasourceReconciler.ReconcileValid()

			//reconcile the refreshing index pipeline
			indexPipelineReconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           createdIndex.Name,
				Version:        createdIndex.Status.RefreshingVersion,
				ConfigFileName: "xjoinindex-with-referenced-field",
				K8sClient:      k8sClient,
				DataSources: []DataSource{{
					Name:                     dataSourceName,
					Version:                  createdDataSource.Status.ActiveVersion,
					ApiCurioResponseFilename: "datasource-latest-version",
				}},
			}
			indexPipelineReconciler.ReconcileUpdated()

			//validate the index's status is updated
			indexReconciler.reconcile()
			updatedIndex := indexReconciler.GetIndex()
			Expect(updatedIndex.Status.RefreshingVersion).To(Equal(""))
			Expect(updatedIndex.Status.RefreshingVersionIsValid).To(Equal(false))
			Expect(updatedIndex.Status.ActiveVersion).To(Equal(createdIndex.Status.RefreshingVersion))
			Expect(updatedIndex.Status.ActiveVersionIsValid).To(Equal(true))
		})

		It("Should create a refreshing pipeline when the active IndexPipeline becomes invalid", func() {
			//setup initial state with an invalid refreshing pipeline
			indexReconciler := IndexTestReconciler{
				Namespace:          namespace,
				Name:               "test-index",
				AvroSchemaFileName: "xjoinindex-with-referenced-field",
				K8sClient:          k8sClient,
			}
			createdIndex := indexReconciler.ReconcileNew()

			Expect(createdIndex.Status.RefreshingVersion).ToNot(Equal(""))
			Expect(createdIndex.Status.RefreshingVersionIsValid).To(Equal(false))
			Expect(createdIndex.Status.ActiveVersion).To(Equal(""))
			Expect(createdIndex.Status.ActiveVersionIsValid).To(Equal(false))

			//create a valid datasource
			dataSourceName := "testdatasource"
			datasourceReconciler := DatasourceTestReconciler{
				Namespace: namespace,
				Name:      dataSourceName,
				K8sClient: k8sClient,
			}
			datasourceReconciler.ReconcileNew()
			createdDataSource := datasourceReconciler.ReconcileValid()

			//reconcile the refreshing index pipeline
			indexPipelineReconciler := XJoinIndexPipelineTestReconciler{
				Namespace:      namespace,
				Name:           createdIndex.Name,
				Version:        createdIndex.Status.RefreshingVersion,
				ConfigFileName: "xjoinindex-with-referenced-field",
				K8sClient:      k8sClient,
				DataSources: []DataSource{{
					Name:                     dataSourceName,
					Version:                  createdDataSource.Status.ActiveVersion,
					ApiCurioResponseFilename: "datasource-latest-version",
				}},
			}
			indexPipelineReconciler.ReconcileUpdated()

			//reconcile the index to the valid state
			indexReconciler.reconcile()
			updatedIndex := indexReconciler.GetIndex()
			Expect(updatedIndex.Status.RefreshingVersion).To(Equal(""))
			Expect(updatedIndex.Status.RefreshingVersionIsValid).To(Equal(false))
			Expect(updatedIndex.Status.ActiveVersion).To(Equal(createdIndex.Status.RefreshingVersion))
			Expect(updatedIndex.Status.ActiveVersionIsValid).To(Equal(true))

			//invalidate the active datasource pipeline
			datasourcePipelineReconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      createdDataSource.GetName() + "." + createdDataSource.Status.ActiveVersion,
				K8sClient: k8sClient,
			}
			datasourcePipelineReconciler.ReconcileInvalid()

			//reconcile the active index pipeline to transition to invalid
			indexPipelineReconciler.ReconcileUpdated()

			//validate the index creates a new refreshing version and invalidates the active version
			indexReconciler.reconcile()
			updatedIndex = indexReconciler.GetIndex()

			Expect(updatedIndex.Status.RefreshingVersion).ToNot(Equal(""))
			Expect(updatedIndex.Status.RefreshingVersionIsValid).To(Equal(false))
			Expect(updatedIndex.Status.ActiveVersion).To(Equal(createdIndex.Status.RefreshingVersion))
			Expect(updatedIndex.Status.ActiveVersionIsValid).To(Equal(false))

			refreshingIndexPipeline := &v1alpha1.XJoinIndexPipeline{}
			indexLookupKey := types.NamespacedName{
				Name:      updatedIndex.GetName() + "." + updatedIndex.Status.RefreshingVersion,
				Namespace: updatedIndex.Namespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), indexLookupKey, refreshingIndexPipeline)
				return err == nil
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

			Expect(refreshingIndexPipeline.Name).To(Equal(
				updatedIndex.GetName() + "." + updatedIndex.Status.RefreshingVersion))
		})
	})
})
