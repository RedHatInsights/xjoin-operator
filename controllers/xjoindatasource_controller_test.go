package controllers_test

import (
	"context"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
				APIVersion:         "xjoin.cloud.redhat.com/v1alpha1",
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

	Context("Reconcile Delete", func() {
		It("Should delete a XJoinDataSourcePipeline", func() {
			reconciler := DatasourceTestReconciler{
				Namespace: namespace,
				Name:      "test-data-source",
				K8sClient: k8sClient,
			}
			createdDataSource := reconciler.ReconcileNew()

			dataSourcePipelineList := &v1alpha1.XJoinDataSourcePipelineList{}
			err := k8sClient.List(context.Background(), dataSourcePipelineList, client.InNamespace(namespace))
			checkError(err)
			Expect(dataSourcePipelineList.Items).To(HaveLen(1))

			err = k8sClient.Delete(context.Background(), &createdDataSource)
			checkError(err)
			reconciler.ReconcileDelete()

			err = k8sClient.List(context.Background(), dataSourcePipelineList, client.InNamespace(namespace))
			checkError(err)
			Expect(dataSourcePipelineList.Items).To(HaveLen(0))
		})
	})

	Context("Pipeline management", func() {
		It("Should update the refreshing status when the refreshing DataSourcePipeline status changes", func() {
			//setup initial state with an invalid refreshing pipeline
			datasourceReconciler := DatasourceTestReconciler{
				Namespace: namespace,
				Name:      "test-data-source",
				K8sClient: k8sClient,
			}
			createdDatasource := datasourceReconciler.ReconcileNew()

			Expect(createdDatasource.Status.RefreshingVersion).ToNot(Equal(""))
			Expect(createdDatasource.Status.RefreshingVersionState.Result).To(Equal(common.New))
			Expect(createdDatasource.Status.ActiveVersion).To(Equal(""))
			Expect(createdDatasource.Status.ActiveVersionState.Result).To(Equal(""))

			//set the refreshing pipeline to valid
			pipelineReconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      createdDatasource.GetName(),
				Version:   createdDatasource.Status.RefreshingVersion,
				K8sClient: k8sClient,
			}
			pipelineReconciler.ReconcileValid()

			//validate the DataSource's status is updated
			datasourceReconciler.reconcile()
			updatedDatasource := datasourceReconciler.GetDataSource()
			Expect(updatedDatasource.Status.RefreshingVersion).To(Equal(""))
			Expect(updatedDatasource.Status.RefreshingVersionState.Result).To(Equal(""))
			Expect(updatedDatasource.Status.ActiveVersion).To(Equal(createdDatasource.Status.RefreshingVersion))
			Expect(updatedDatasource.Status.ActiveVersionState.Result).To(Equal(common.Valid))
		})

		It("Should update the active pipeline status when the active DataSourcePipeline status changes", func() {
			//setup initial state with an invalid refreshing pipeline
			datasourceReconciler := DatasourceTestReconciler{
				Namespace: namespace,
				Name:      "test-data-source",
				K8sClient: k8sClient,
			}
			createdDatasource := datasourceReconciler.ReconcileNew()

			//set the refreshing pipeline to valid
			pipelineReconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      createdDatasource.GetName(),
				Version:   createdDatasource.Status.RefreshingVersion,
				K8sClient: k8sClient,
			}
			pipelineReconciler.ReconcileValid()

			//validate the DataSource's status is in the correct state
			datasourceReconciler.reconcile()
			updatedDatasource := datasourceReconciler.GetDataSource()
			Expect(updatedDatasource.Status.RefreshingVersion).To(Equal(""))
			Expect(updatedDatasource.Status.RefreshingVersionState.Result).To(Equal(""))
			Expect(updatedDatasource.Status.ActiveVersion).To(Equal(createdDatasource.Status.RefreshingVersion))
			Expect(updatedDatasource.Status.ActiveVersionState.Result).To(Equal(common.Valid))

			//set the active pipeline to invalid
			activePipelineReconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      createdDatasource.GetName(),
				Version:   updatedDatasource.Status.ActiveVersion,
				K8sClient: k8sClient,
			}
			activePipelineReconciler.ReconcileInvalid()

			//validate the DataSource's status is updated
			datasourceReconciler.reconcile()
			updatedDatasource = datasourceReconciler.GetDataSource()
			Expect(updatedDatasource.Status.RefreshingVersion).ToNot(Equal(""))
			Expect(updatedDatasource.Status.RefreshingVersionState.Result).To(Equal(common.New))
			Expect(updatedDatasource.Status.ActiveVersion).To(Equal(createdDatasource.Status.RefreshingVersion))
			Expect(updatedDatasource.Status.ActiveVersionState.Result).To(Equal(common.Invalid))
		})

		It("Should replace the active pipeline with the refreshing DataSourcePipeline when it becomes valid", func() {
			//setup initial state with an invalid refreshing pipeline
			datasourceReconciler := DatasourceTestReconciler{
				Namespace: namespace,
				Name:      "test-data-source",
				K8sClient: k8sClient,
			}
			createdDatasource := datasourceReconciler.ReconcileNew()

			//set the refreshing pipeline to valid
			pipelineReconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      createdDatasource.GetName(),
				Version:   createdDatasource.Status.RefreshingVersion,
				K8sClient: k8sClient,
			}
			pipelineReconciler.ReconcileValid()

			//validate the DataSource's active pipeline is invalid with no refreshing pipeline
			datasourceReconciler.reconcile()
			updatedDatasource := datasourceReconciler.GetDataSource()
			Expect(updatedDatasource.Status.RefreshingVersion).To(Equal(""))
			Expect(updatedDatasource.Status.RefreshingVersionState.Result).To(Equal(""))
			Expect(updatedDatasource.Status.ActiveVersion).To(Equal(createdDatasource.Status.RefreshingVersion))
			Expect(updatedDatasource.Status.ActiveVersionState.Result).To(Equal(common.Valid))

			//set the active pipeline to invalid
			activePipelineReconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      createdDatasource.GetName(),
				Version:   updatedDatasource.Status.ActiveVersion,
				K8sClient: k8sClient,
			}
			activePipelineReconciler.ReconcileInvalid()

			//validate the DataSource's status is in the refreshing state with an invalid active pipeline
			datasourceReconciler.reconcile()
			updatedDatasource = datasourceReconciler.GetDataSource()
			Expect(updatedDatasource.Status.RefreshingVersion).ToNot(Equal(""))
			Expect(updatedDatasource.Status.RefreshingVersionState.Result).To(Equal(common.New))
			Expect(updatedDatasource.Status.ActiveVersion).To(Equal(createdDatasource.Status.RefreshingVersion))
			Expect(updatedDatasource.Status.ActiveVersionState.Result).To(Equal(common.Invalid))

			//set the refreshing pipeline to valid
			refreshingPipelineReconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      createdDatasource.GetName(),
				Version:   updatedDatasource.Status.RefreshingVersion,
				K8sClient: k8sClient,
			}
			refreshingPipelineReconciler.ReconcileValid()

			//validate the DataSource's status is updated
			refreshingVersion := updatedDatasource.Status.RefreshingVersion
			datasourceReconciler.reconcile()
			updatedDatasource = datasourceReconciler.GetDataSource()
			Expect(updatedDatasource.Status.RefreshingVersion).To(Equal(""))
			Expect(updatedDatasource.Status.RefreshingVersionState.Result).To(Equal(""))
			Expect(updatedDatasource.Status.ActiveVersion).To(Equal(refreshingVersion))
			Expect(updatedDatasource.Status.ActiveVersionState.Result).To(Equal(common.Valid))
		})

		It("Should create a refreshing pipeline when the active DataSourcePipeline becomes invalid", func() {
			//setup initial state with an invalid refreshing pipeline
			datasourceReconciler := DatasourceTestReconciler{
				Namespace: namespace,
				Name:      "test-data-source",
				K8sClient: k8sClient,
			}
			createdDatasource := datasourceReconciler.ReconcileNew()

			//set the refreshing pipeline to valid
			pipelineReconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      createdDatasource.GetName(),
				Version:   createdDatasource.Status.RefreshingVersion,
				K8sClient: k8sClient,
			}
			pipelineReconciler.ReconcileValid()

			//validate the DataSource's status is in the correct state
			datasourceReconciler.reconcile()
			updatedDatasource := datasourceReconciler.GetDataSource()
			Expect(updatedDatasource.Status.RefreshingVersion).To(Equal(""))
			Expect(updatedDatasource.Status.RefreshingVersionState.Result).To(Equal(""))
			Expect(updatedDatasource.Status.ActiveVersion).To(Equal(createdDatasource.Status.RefreshingVersion))
			Expect(updatedDatasource.Status.ActiveVersionState.Result).To(Equal(common.Valid))

			//set the active pipeline to invalid
			activePipelineReconciler := DatasourcePipelineTestReconciler{
				Namespace: namespace,
				Name:      createdDatasource.GetName(),
				Version:   updatedDatasource.Status.ActiveVersion,
				K8sClient: k8sClient,
			}
			activePipelineReconciler.ReconcileInvalid()

			//validate the DataSource's status is updated
			datasourceReconciler.reconcile()
			updatedDatasource = datasourceReconciler.GetDataSource()

			Expect(updatedDatasource.Status.RefreshingVersion).ToNot(Equal(""))
			Expect(updatedDatasource.Status.RefreshingVersionState.Result).To(Equal(common.New))
			Expect(updatedDatasource.Status.ActiveVersion).To(Equal(createdDatasource.Status.RefreshingVersion))
			Expect(updatedDatasource.Status.ActiveVersionState.Result).To(Equal(common.Invalid))

			//validate the refreshing pipeline was created
			refreshingPipeline := &v1alpha1.XJoinDataSourcePipeline{}
			datasourceLookupKey := types.NamespacedName{
				Name:      updatedDatasource.GetName() + "." + updatedDatasource.Status.RefreshingVersion,
				Namespace: updatedDatasource.Namespace,
			}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), datasourceLookupKey, refreshingPipeline)
				return err == nil
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

			Expect(refreshingPipeline.Name).To(Equal(
				updatedDatasource.GetName() + "." + updatedDatasource.Status.RefreshingVersion))
		})
	})

	It("Should create create a new refreshing pipeline when refreshing fails", func() {
		datasourceReconciler := DatasourceTestReconciler{
			Namespace: namespace,
			Name:      "test-data-source",
			K8sClient: k8sClient,
		}
		createdDatasource := datasourceReconciler.ReconcileNew()

		//set the refreshing pipeline to invalid
		pipelineReconciler := DatasourcePipelineTestReconciler{
			Namespace: namespace,
			Name:      createdDatasource.GetName(),
			Version:   createdDatasource.Status.RefreshingVersion,
			K8sClient: k8sClient,
		}
		pipelineReconciler.ReconcileInvalid()

		//reconcile the datasource to first trigger the refresh failed state
		updatedDatasource := datasourceReconciler.ReconcileUpdated()
		Expect(updatedDatasource.Status.RefreshingVersion).To(BeEmpty())
		Expect(updatedDatasource.Status.RefreshingVersionState.Result).To(BeEmpty())

		//reconcile the datasourcePipeline to trigger deletion
		pipelineReconciler.ReconcileDelete()

		//reconcile the datasource again to trigger the new refreshing state
		updatedDatasource = datasourceReconciler.ReconcileUpdated()
		Expect(updatedDatasource.Status.RefreshingVersion).ToNot(BeEmpty())
		Expect(createdDatasource.Status.RefreshingVersion).ToNot(BeEmpty())
		Expect(updatedDatasource.Status.RefreshingVersion).ToNot(Equal(createdDatasource.Status.RefreshingVersion))
		Expect(updatedDatasource.Status.RefreshingVersionState.Result).To(Equal(common.New))
	})
})
