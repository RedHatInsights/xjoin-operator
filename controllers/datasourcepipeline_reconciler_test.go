package controllers_test

import (
	"context"
	"fmt"
	"github.com/redhatinsights/xjoin-operator/controllers/common"

	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
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

type DatasourcePipelineTestReconciler struct {
	Namespace          string
	Name               string
	Version            string
	K8sClient          client.Client
	AvroSchemaFileName string
}

func (d *DatasourcePipelineTestReconciler) fullName() string {
	return d.Name + "." + d.Version
}

func (d *DatasourcePipelineTestReconciler) getNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: d.Namespace,
		Name:      d.Name + "." + d.Version,
	}
}

func (d *DatasourcePipelineTestReconciler) newXJoinDataSourcePipelineReconciler() *controllers.XJoinDataSourcePipelineReconciler {
	return controllers.NewXJoinDataSourcePipelineReconciler(
		d.K8sClient,
		scheme.Scheme,
		testLogger,
		record.NewFakeRecorder(100),
		d.Namespace,
		true)
}

func (d *DatasourcePipelineTestReconciler) CreateValidDataSourcePipeline() {
	ctx := context.Background()

	//optionally load avro schema from file
	var datasourceAvroSchema string
	if d.AvroSchemaFileName != "" {
		datasourceAvroSchemaBytes, err := os.ReadFile("./test/data/avro/" + d.AvroSchemaFileName + ".json")
		Expect(err).ToNot(HaveOccurred())
		datasourceAvroSchema = string(datasourceAvroSchemaBytes)
	} else {
		datasourceAvroSchema = "{}"
	}

	datasourcePipelineSpec := v1alpha1.XJoinDataSourcePipelineSpec{
		Name:             d.Name,
		Version:          d.Version,
		AvroSchema:       datasourceAvroSchema,
		DatabaseHostname: &v1alpha1.StringOrSecretParameter{Value: "dbHost"},
		DatabasePort:     &v1alpha1.StringOrSecretParameter{Value: "8080"},
		DatabaseUsername: &v1alpha1.StringOrSecretParameter{Value: "dbUsername"},
		DatabasePassword: &v1alpha1.StringOrSecretParameter{Value: "dbPassword"},
		DatabaseName:     &v1alpha1.StringOrSecretParameter{Value: "dbName"},
		DatabaseTable:    &v1alpha1.StringOrSecretParameter{Value: "dbTable"},
		Pause:            false,
	}

	datasourcePipeline := &v1alpha1.XJoinDataSourcePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.fullName(),
			Namespace: d.Namespace,
		},
		Spec: datasourcePipelineSpec,
		TypeMeta: metav1.TypeMeta{
			APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
			Kind:       "XJoinDataSourcePipeline",
		},
	}

	Expect(d.K8sClient.Create(ctx, datasourcePipeline)).Should(Succeed())

	//validate datasource spec is created correctly
	createdDataSourcePipeline := &v1alpha1.XJoinDataSourcePipeline{}

	Eventually(func() bool {
		err := d.K8sClient.Get(ctx, d.getNamespacedName(), createdDataSourcePipeline)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())

	Expect(createdDataSourcePipeline.Spec.Name).Should(Equal(d.Name))
	Expect(createdDataSourcePipeline.Spec.Version).Should(Equal(d.Version))
	Expect(createdDataSourcePipeline.Spec.Pause).Should(Equal(false))
	Expect(createdDataSourcePipeline.Spec.AvroSchema).Should(Equal(datasourceAvroSchema))
	Expect(createdDataSourcePipeline.Spec.DatabaseHostname).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbHost"}))
	Expect(createdDataSourcePipeline.Spec.DatabasePort).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "8080"}))
	Expect(createdDataSourcePipeline.Spec.DatabaseUsername).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbUsername"}))
	Expect(createdDataSourcePipeline.Spec.DatabasePassword).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbPassword"}))
	Expect(createdDataSourcePipeline.Spec.DatabaseName).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbName"}))
	Expect(createdDataSourcePipeline.Spec.DatabaseTable).Should(Equal(&v1alpha1.StringOrSecretParameter{Value: "dbTable"}))
}

func (d *DatasourcePipelineTestReconciler) ReconcileNew() v1alpha1.XJoinDataSourcePipeline {
	d.registerNewMocks()
	d.CreateValidDataSourcePipeline()
	createdDataSourcePipeline := &v1alpha1.XJoinDataSourcePipeline{}
	result := d.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

	Eventually(func() bool {
		err := d.K8sClient.Get(context.Background(), d.getNamespacedName(), createdDataSourcePipeline)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

	return *createdDataSourcePipeline
}

func (d *DatasourcePipelineTestReconciler) ReconcileDelete() {
	d.registerDeleteMocks()
	result := d.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 0}))

	datasourcePipelineList := v1alpha1.XJoinDataSourcePipelineList{}
	err := d.K8sClient.List(context.Background(), &datasourcePipelineList, client.InNamespace(d.Namespace))
	checkError(err)
	Expect(datasourcePipelineList.Items).To(HaveLen(0))
}

func (d *DatasourcePipelineTestReconciler) ReconcileValid() v1alpha1.XJoinDataSourcePipeline {
	d.registerValidMocks()
	result := d.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

	createdDataSourcePipeline := &v1alpha1.XJoinDataSourcePipeline{}
	Eventually(func() bool {
		err := d.K8sClient.Get(context.Background(), d.getNamespacedName(), createdDataSourcePipeline)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

	createdDataSourcePipeline.Status.ValidationResponse = validation.ValidationResponse{
		Result: common.Valid,
	}
	err := d.K8sClient.Status().Update(context.Background(), createdDataSourcePipeline)
	Expect(err).ToNot(HaveOccurred())

	return *createdDataSourcePipeline
}

func (d *DatasourcePipelineTestReconciler) ReconcileInvalid() v1alpha1.XJoinDataSourcePipeline {
	d.registerValidMocks()
	result := d.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

	createdDataSourcePipeline := &v1alpha1.XJoinDataSourcePipeline{}
	Eventually(func() bool {
		err := d.K8sClient.Get(context.Background(), d.getNamespacedName(), createdDataSourcePipeline)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

	createdDataSourcePipeline.Status.ValidationResponse = validation.ValidationResponse{
		Result: common.Invalid,
	}
	err := d.K8sClient.Status().Update(context.Background(), createdDataSourcePipeline)
	Expect(err).ToNot(HaveOccurred())

	return *createdDataSourcePipeline
}

func (d *DatasourcePipelineTestReconciler) ReconcileUpdated() v1alpha1.XJoinDataSourcePipeline {
	d.registerUpdatedMocks()
	result := d.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

	createdDataSourcePipeline := &v1alpha1.XJoinDataSourcePipeline{}
	Eventually(func() bool {
		err := d.K8sClient.Get(context.Background(), d.getNamespacedName(), createdDataSourcePipeline)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

	return *createdDataSourcePipeline
}

func (d *DatasourcePipelineTestReconciler) reconcile() reconcile.Result {
	xjoinDataSourcePipelineReconciler := d.newXJoinDataSourcePipelineReconciler()
	result, err := xjoinDataSourcePipelineReconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: d.getNamespacedName()})
	checkError(err)
	return result
}

func (d *DatasourcePipelineTestReconciler) registerDeleteMocks() {
	httpmock.Reset()
	httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

	//avro schema mocks
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-value/versions/1",
		httpmock.NewStringResponder(200, `{}`))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-value/versions/latest",
		httpmock.NewStringResponder(200, `{}`))

	httpmock.RegisterResponder(
		"DELETE",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-value",
		httpmock.NewStringResponder(200, `{}`))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-key/versions/latest",
		httpmock.NewStringResponder(200, `{}`))

	httpmock.RegisterResponder(
		"DELETE",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-key",
		httpmock.NewStringResponder(200, `{}`))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/registry/v2/groups/default/artifacts/xjoindatasourcepipeline."+d.fullName()+"/versions",
		httpmock.NewStringResponder(404, `{}`))

	//kafka connector mocks
	httpmock.RegisterResponder(
		"GET",
		"http://connect-connect-api."+d.Namespace+".svc:8083/connectors/xjoindatasourcepipeline."+d.fullName(),
		httpmock.NewStringResponder(404, `{}`))
}

func (d *DatasourcePipelineTestReconciler) registerNewMocks() {
	httpmock.Reset()
	httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

	//avro schema mocks
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-value/versions/1",
		httpmock.NewStringResponder(404, `{"message":"No version '1' found for artifact with ID 'xjoindatasourcepipeline.`+d.fullName()+`-value' in group 'null'.","error_code":40402}`).Times(1))

	httpmock.RegisterResponder(
		"POST",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-value/versions",
		httpmock.NewStringResponder(200, `{"createdBy":"","createdOn":"2022-07-27T17:28:11+0000","modifiedBy":"","modifiedOn":"2022-07-27T17:28:11+0000","id":1,"version":1,"type":"AVRO","globalId":1,"state":"ENABLED","groupId":"null","contentId":1,"references":[]}`))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/schemas/ids/1",
		httpmock.NewStringResponder(200, `{"schema":"{\"name\":\"Value\",\"namespace\":\"xjoindatasourcepipeline.`+d.fullName()+`\"}","schemaType":"AVRO","references":[]}`))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-value/versions/latest",
		httpmock.NewStringResponder(200, "{}"))
}

func (d *DatasourcePipelineTestReconciler) registerValidMocks() {
	httpmock.Reset()
	httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

	//avro schema mocks
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-value/versions/1",
		httpmock.NewStringResponder(200, fmt.Sprintf(
			`{"id": 1, "subject": "%s", "version": 1, "references": [], "schema": "{\"name\":\"Value\",\"namespace\":\"%s\"}"}`,
			"xjoindatasourcepipeline."+d.Name+"-value", "xjoindatasourcepipeline."+d.Name)))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-value/versions/latest",
		httpmock.NewStringResponder(200, fmt.Sprintf(
			`{"id": 1, "subject": "%s", "version": 1, "references": [], "schema": "{\"name\":\"Value\",\"namespace\":\"%s\"}"}`,
			"xjoindatasourcepipeline."+d.Name+"-value", "xjoindatasourcepipeline."+d.Name)))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects",
		httpmock.NewStringResponder(200, "[]"))
}

func (d *DatasourcePipelineTestReconciler) registerUpdatedMocks() {
	httpmock.Reset()
	httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

	//avro schema mocks
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-value/versions/1",
		httpmock.NewStringResponder(200, fmt.Sprintf(
			`{"id": 1, "subject": "%s", "version": 1, "references": [], "schema": "{\"name\":\"Value\",\"namespace\":\"%s\"}"}`,
			"xjoindatasourcepipeline."+d.Name+"-value", "xjoindatasourcepipeline."+d.Name)))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline."+d.fullName()+"-value/versions/latest",
		httpmock.NewStringResponder(200, fmt.Sprintf(
			`{"id": 1, "subject": "%s", "version": 1, "references": [], "schema": "{\"name\":\"Value\",\"namespace\":\"%s\"}"}`,
			"xjoindatasourcepipeline."+d.Name+"-value", "xjoindatasourcepipeline."+d.Name)))

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects",
		httpmock.NewStringResponder(200, "[]"))
}
