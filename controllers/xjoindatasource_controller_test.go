package controllers

import (
	"context"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("XJoinDataSource", func() {
	var namespace string

	var newXJoinDataSourceReconciler = func() *XJoinDataSourceReconciler {
		return NewXJoinDataSourceReconciler(
			k8sClient,
			scheme.Scheme,
			testLogger,
			record.NewFakeRecorder(10),
			namespace,
			true)
	}

	var createValidDataSource = func(name string) {
		ctx := context.Background()

		httpmock.RegisterResponder(
			"GET",
			"http://example-apicurioregistry-kafkasql-service.test.svc:1080/apis/ccompat/v6/subjects",
			httpmock.NewStringResponder(200, `[]`))

		datasourceSpec := v1alpha1.XJoinDataSourceSpec{
			AvroSchema:       "{}",
			DatabaseHostname: &v1alpha1.StringOrSecretParameter{Value: "dbHost"},
			DatabasePort:     &v1alpha1.StringOrSecretParameter{Value: "8080"},
			DatabaseUsername: &v1alpha1.StringOrSecretParameter{Value: "dbUsername"},
			DatabasePassword: &v1alpha1.StringOrSecretParameter{Value: "dbPassword"},
			DatabaseName:     &v1alpha1.StringOrSecretParameter{Value: "dbName"},
			DatabaseTable:    &v1alpha1.StringOrSecretParameter{Value: "dbTable"},
			Pause:            false,
		}

		datasource := &v1alpha1.XJoinDataSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: datasourceSpec,
			TypeMeta: metav1.TypeMeta{
				APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
				Kind:       "XJoinDataSource",
			},
		}

		Expect(k8sClient.Create(ctx, datasource)).Should(Succeed())

		//validate datasource spec is created correctly
		datasourceLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
		createdDatasource := &v1alpha1.XJoinDataSource{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, datasourceLookupKey, createdDatasource)
			if err != nil {
				return false
			}
			return true
		}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
		Expect(createdDatasource.Spec.Pause).Should(Equal(false))
		Expect(createdDatasource.Spec.AvroSchema).Should(Equal("{}"))
		Expect(createdDatasource.Spec.DatabaseHostname).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbHost"}))
		Expect(createdDatasource.Spec.DatabasePort).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "8080"}))
		Expect(createdDatasource.Spec.DatabaseUsername).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbUsername"}))
		Expect(createdDatasource.Spec.DatabasePassword).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbPassword"}))
		Expect(createdDatasource.Spec.DatabaseName).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbName"}))
		Expect(createdDatasource.Spec.DatabaseTable).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbTable"}))
	}

	var reconcileDataSource = func(name string) v1alpha1.XJoinDataSource {
		ctx := context.Background()

		xjoinDataSourceReconciler := newXJoinDataSourceReconciler()

		datasourceLookupKey := types.NamespacedName{Name: name, Namespace: namespace}
		createdDatasource := &v1alpha1.XJoinDataSource{}

		result, err := xjoinDataSourceReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: datasourceLookupKey})
		checkError(err)
		Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

		Eventually(func() bool {
			err := k8sClient.Get(ctx, datasourceLookupKey, createdDatasource)
			if err != nil {
				return false
			}
			return true
		}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
		Expect(createdDatasource.Status.ActiveVersion).To(Equal(""))
		Expect(createdDatasource.Status.ActiveVersionIsValid).To(Equal(false))
		Expect(createdDatasource.Status.RefreshingVersion).ToNot(Equal(""))
		Expect(createdDatasource.Status.RefreshingVersionIsValid).To(Equal(false))
		Expect(createdDatasource.Status.SpecHash).ToNot(Equal(""))
		Expect(createdDatasource.Finalizers).To(HaveLen(1))
		Expect(createdDatasource.Finalizers).To(ContainElement("finalizer.xjoin.datasource.cloud.redhat.com"))

		info := httpmock.GetCallCountInfo()
		count := info["GET http://example-apicurioregistry-kafkasql-service.test.svc:1080/apis/ccompat/v6/subjects"]
		Expect(count).To(Equal(1))

		return *createdDatasource
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
		It("Should create a XJoinDataSourcePipeline", func() {
			dataSourceName := "test-data-source"
			createValidDataSource(dataSourceName)
			createdDataSource := reconcileDataSource(dataSourceName)
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
