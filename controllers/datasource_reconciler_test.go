package controllers_test

import (
	"context"
	"github.com/redhatinsights/xjoin-operator/controllers"
	"os"
	"time"

	"github.com/jarcoal/httpmock"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type DatasourceTestReconciler struct {
	Namespace          string
	Name               string
	K8sClient          client.Client
	AvroSchemaFileName string
	createdDatasource  v1alpha1.XJoinDataSource
}

func (d *DatasourceTestReconciler) ReconcileNew() v1alpha1.XJoinDataSource {
	d.registerNewMocks()
	d.createValidDataSource()
	result := d.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

	createdDatasource := &v1alpha1.XJoinDataSource{}
	datasourceLookupKey := types.NamespacedName{Name: d.Name, Namespace: d.Namespace}

	Eventually(func() bool {
		err := d.K8sClient.Get(context.Background(), datasourceLookupKey, createdDatasource)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())

	Expect(createdDatasource.Status.ActiveVersion).To(Equal(""))
	Expect(createdDatasource.Status.ActiveVersionIsValid).To(Equal(false))
	Expect(createdDatasource.Status.RefreshingVersion).ToNot(Equal(""))
	Expect(createdDatasource.Status.RefreshingVersionIsValid).To(Equal(false))
	Expect(createdDatasource.Status.SpecHash).ToNot(Equal(""))
	Expect(createdDatasource.Finalizers).To(HaveLen(1))
	Expect(createdDatasource.Finalizers).To(ContainElement("finalizer.xjoin.datasource.cloud.redhat.com"))

	info := httpmock.GetCallCountInfo()
	count := info["GET http://apicurio:1080/apis/ccompat/v6/subjects"]
	Expect(count).To(Equal(1))

	d.createdDatasource = *createdDatasource
	return *createdDatasource
}

func (d *DatasourceTestReconciler) ReconcileValid() v1alpha1.XJoinDataSource {
	//set the refreshing pipeline to valid
	datasourcePipelineReconciler := DatasourcePipelineTestReconciler{
		Version:   d.createdDatasource.Status.RefreshingVersion,
		Namespace: d.Namespace,
		Name:      d.Name,
		K8sClient: k8sClient,
	}
	datasourcePipelineReconciler.ReconcileValid()

	//assert datasource is in valid state
	d.reconcile()
	result := d.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

	updatedDatasource := &v1alpha1.XJoinDataSource{}
	datasourceLookupKey := types.NamespacedName{Name: d.Name, Namespace: d.Namespace}

	Eventually(func() bool {
		err := d.K8sClient.Get(context.Background(), datasourceLookupKey, updatedDatasource)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
	Expect(updatedDatasource.Status.ActiveVersion).ToNot(Equal(""))
	Expect(updatedDatasource.Status.ActiveVersionIsValid).To(Equal(true))
	Expect(updatedDatasource.Status.RefreshingVersion).To(Equal(""))
	Expect(updatedDatasource.Status.RefreshingVersionIsValid).To(Equal(false))
	Expect(updatedDatasource.Status.SpecHash).ToNot(Equal(""))
	Expect(updatedDatasource.Finalizers).To(HaveLen(1))
	Expect(updatedDatasource.Finalizers).To(ContainElement("finalizer.xjoin.datasource.cloud.redhat.com"))

	info := httpmock.GetCallCountInfo()
	count := info["GET http://apicurio:1080/apis/ccompat/v6/subjects"]
	Expect(count).To(Equal(2))

	return *updatedDatasource
}

func (d *DatasourceTestReconciler) ReconcileDelete() {
	d.registerDeleteMocks()
	result := d.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 0}))

	datasourceList := v1alpha1.XJoinDataSourceList{}
	err := d.K8sClient.List(context.Background(), &datasourceList, client.InNamespace(d.Namespace))
	checkError(err)
	Expect(datasourceList.Items).To(HaveLen(0))
}

func (d *DatasourceTestReconciler) GetDataSource() v1alpha1.XJoinDataSource {
	dataSource := &v1alpha1.XJoinDataSource{}
	datasourceLookupKey := types.NamespacedName{Name: d.Name, Namespace: d.Namespace}
	Eventually(func() bool {
		err := d.K8sClient.Get(context.Background(), datasourceLookupKey, dataSource)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())
	return *dataSource
}

func (d *DatasourceTestReconciler) createValidDataSource() {
	ctx := context.Background()

	var datasourceAvroSchema string
	if d.AvroSchemaFileName != "" {
		datasourceAvroSchemaBytes, err := os.ReadFile("./test/data/avro/" + d.AvroSchemaFileName + ".json")
		Expect(err).ToNot(HaveOccurred())
		datasourceAvroSchema = string(datasourceAvroSchemaBytes)
	} else {
		datasourceAvroSchema = "{}"
	}

	datasourceSpec := v1alpha1.XJoinDataSourceSpec{
		AvroSchema:       datasourceAvroSchema,
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
			Name:      d.Name,
			Namespace: d.Namespace,
		},
		Spec: datasourceSpec,
		TypeMeta: metav1.TypeMeta{
			APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
			Kind:       "XJoinDataSource",
		},
	}

	Expect(d.K8sClient.Create(ctx, datasource)).Should(Succeed())

	//validate datasource spec is created correctly
	datasourceLookupKey := types.NamespacedName{Name: d.Name, Namespace: d.Namespace}
	createdDatasource := &v1alpha1.XJoinDataSource{}

	Eventually(func() bool {
		err := d.K8sClient.Get(ctx, datasourceLookupKey, createdDatasource)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
	Expect(createdDatasource.Spec.Pause).Should(Equal(false))
	Expect(createdDatasource.Spec.AvroSchema).Should(Equal(datasourceAvroSchema))
	Expect(createdDatasource.Spec.DatabaseHostname).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbHost"}))
	Expect(createdDatasource.Spec.DatabasePort).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "8080"}))
	Expect(createdDatasource.Spec.DatabaseUsername).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbUsername"}))
	Expect(createdDatasource.Spec.DatabasePassword).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbPassword"}))
	Expect(createdDatasource.Spec.DatabaseName).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbName"}))
	Expect(createdDatasource.Spec.DatabaseTable).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbTable"}))
}

func (d *DatasourceTestReconciler) reconcile() reconcile.Result {
	xjoinDataSourceReconciler := d.newXJoinDataSourceReconciler()
	datasourceLookupKey := types.NamespacedName{Name: d.Name, Namespace: d.Namespace}
	result, err := xjoinDataSourceReconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: datasourceLookupKey})
	checkError(err)
	return result
}

func (d *DatasourceTestReconciler) registerNewMocks() {
	httpmock.Reset()
	httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects",
		httpmock.NewStringResponder(200, `[]`))
}

func (d *DatasourceTestReconciler) registerDeleteMocks() {
	httpmock.Reset()
	httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects",
		httpmock.NewStringResponder(200, `[]`))
}

func (d *DatasourceTestReconciler) newXJoinDataSourceReconciler() *controllers.XJoinDataSourceReconciler {
	return controllers.NewXJoinDataSourceReconciler(
		d.K8sClient,
		scheme.Scheme,
		testLogger,
		record.NewFakeRecorder(100),
		d.Namespace,
		true)
}
