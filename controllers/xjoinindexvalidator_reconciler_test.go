package controllers_test

import (
	"context"
	"fmt"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type XJoinIndexValidatorTestReconciler struct {
	Namespace             string
	Name                  string
	Version               string
	K8sClient             client.Client
	ConfigFileName        string
	createdIndexValidator v1alpha1.XJoinIndexValidator
	PodLogReader          k8s.LogReader
}

func (x *XJoinIndexValidatorTestReconciler) GetName() string {
	return x.Name + "." + x.Version
}

func (x *XJoinIndexValidatorTestReconciler) ReconcileCreate() (v1alpha1.XJoinIndexValidator, reconcile.Result) {
	x.createIndexValidator()
	result := x.reconcile()
	validatorLookupKey := types.NamespacedName{Name: x.GetName(), Namespace: x.Namespace}
	Eventually(func() bool {
		err := x.K8sClient.Get(context.Background(), validatorLookupKey, &x.createdIndexValidator)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())
	return x.createdIndexValidator, result
}

func (x *XJoinIndexValidatorTestReconciler) ReconcileRunning() (v1alpha1.XJoinIndexValidator, reconcile.Result) {
	x.registerRunningMocks()
	result := x.reconcile()
	return x.createdIndexValidator, result
}

func (x *XJoinIndexValidatorTestReconciler) ReconcileSuccess() (validator v1alpha1.XJoinIndexValidator, result reconcile.Result) {
	x.registerSuccessMocks()
	validatorPods := x.ListValidatorPods()
	validatorPods.Items[0].Status.Phase = corev1.PodSucceeded
	err := x.K8sClient.Status().Update(context.Background(), &validatorPods.Items[0])
	newPods := x.ListValidatorPods()
	newPods.Items[0].GetNamespace()
	checkError(err)

	result = x.reconcile()
	validatorLookupKey := types.NamespacedName{Name: x.GetName(), Namespace: x.Namespace}
	Eventually(func() bool {
		err := x.K8sClient.Get(context.Background(), validatorLookupKey, &validator)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

	return
}

func (x *XJoinIndexValidatorTestReconciler) ReconcileFailure() (validator v1alpha1.XJoinIndexValidator, result reconcile.Result) {
	x.registerFailureMocks()
	validatorPods := x.ListValidatorPods()
	validatorPods.Items[0].Status.Phase = corev1.PodFailed
	err := x.K8sClient.Status().Update(context.Background(), &validatorPods.Items[0])
	newPods := x.ListValidatorPods()
	newPods.Items[0].GetNamespace()
	checkError(err)

	result = x.reconcile()
	validatorLookupKey := types.NamespacedName{Name: x.GetName(), Namespace: x.Namespace}
	Eventually(func() bool {
		err := x.K8sClient.Get(context.Background(), validatorLookupKey, &validator)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

	return
}

func (x *XJoinIndexValidatorTestReconciler) ListValidatorPods() *corev1.PodList {
	labels := client.MatchingLabels{}
	labels["xjoin.index"] = x.GetName()
	labels[common.COMPONENT_NAME_LABEL] = "XJoinIndexValidator"

	pods := &corev1.PodList{}
	err := k8sClient.List(context.Background(), pods, client.InNamespace(x.Namespace), labels)
	checkError(err)
	return pods
}

func (x *XJoinIndexValidatorTestReconciler) CreateDatasource() {
	reconciler := DatasourceTestReconciler{
		Namespace:          x.Namespace,
		Name:               "testdatasource",
		K8sClient:          k8sClient,
		AvroSchemaFileName: "xjoindatasource-single-field",
	}
	createdDataSource := reconciler.ReconcileNew()
	Expect(createdDataSource.Name).To(Equal("testdatasource"))
	refreshingVersion := createdDataSource.Status.RefreshingVersion
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoindatasourcepipeline.testdatasource."+refreshingVersion+"-value/versions/latest",
		httpmock.NewStringResponder(200, fmt.Sprintf(
			`{"id": 1, "subject": "xjoindatasourcepipeline.testdatasource.%s-value", "version": 1, "schema": "%s", "references": []}`, refreshingVersion, "{}")))
}

func (x *XJoinIndexValidatorTestReconciler) createIndexValidator() {
	x.registerCreateMocks()
	ctx := context.Background()
	indexAvroSchema, err := os.ReadFile("./test/data/avro/" + x.ConfigFileName + ".json")
	checkError(err)

	//XjoinIndexValidator requires an XJoinIndexPipeline owner. Create one here
	indexPipelineSpec := v1alpha1.XJoinIndexPipelineSpec{
		AvroSchema: string(indexAvroSchema),
		Version:    x.Version,
		Pause:      false,
	}

	xjoinIndexPipelineName := "test-index-pipeline"

	index := &v1alpha1.XJoinIndexPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      xjoinIndexPipelineName,
			Namespace: x.Namespace,
		},
		Spec: indexPipelineSpec,
		TypeMeta: metav1.TypeMeta{
			APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
			Kind:       "XJoinIndexPipeline",
		},
	}

	Expect(x.K8sClient.Create(ctx, index)).Should(Succeed())

	//create the XJoinIndexValidator
	validatorIndexName := "xjoinindexpipeline." + x.GetName()
	indexValidatorSpec := v1alpha1.XJoinIndexValidatorSpec{
		Name:       x.Name,
		Version:    x.Version,
		AvroSchema: string(indexAvroSchema),
		Pause:      false,
		IndexName:  validatorIndexName,
	}

	blockOwnerDeletion := true
	controller := true
	indexValidator := &v1alpha1.XJoinIndexValidator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      x.GetName(),
			Namespace: x.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         common.IndexPipelineGVK.Version,
					Kind:               common.IndexPipelineGVK.Kind,
					Name:               xjoinIndexPipelineName,
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
					UID:                "a6778b9b-dfed-4d41-af53-5ebbcddb7535",
				},
			},
		},
		Spec: indexValidatorSpec,
		TypeMeta: metav1.TypeMeta{
			APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
			Kind:       "XJoinIndexValidator",
		},
	}

	Expect(x.K8sClient.Create(ctx, indexValidator)).Should(Succeed())

	//validate indexValidator spec is created correctly
	indexValidatorLookupKey := types.NamespacedName{Name: x.GetName(), Namespace: x.Namespace}
	createdIndexValidator := &v1alpha1.XJoinIndexValidator{}

	Eventually(func() bool {
		err := x.K8sClient.Get(ctx, indexValidatorLookupKey, createdIndexValidator)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

	Expect(createdIndexValidator.Spec.Name).Should(Equal(x.Name))
	Expect(createdIndexValidator.Spec.Version).Should(Equal(x.Version))
	Expect(createdIndexValidator.Spec.Pause).Should(Equal(false))
	Expect(createdIndexValidator.Spec.AvroSchema).Should(Equal(string(indexAvroSchema)))
	Expect(createdIndexValidator.Spec.IndexName).Should(Equal(validatorIndexName))
}

func (x *XJoinIndexValidatorTestReconciler) newXJoinIndexValidatorReconciler() *controllers.XJoinIndexValidatorReconciler {
	return controllers.NewXJoinIndexValidatorReconciler(
		x.K8sClient,
		scheme.Scheme,
		fake.NewSimpleClientset(),
		testLogger,
		record.NewFakeRecorder(100),
		x.Namespace,
		true,
		x.PodLogReader)
}

func (x *XJoinIndexValidatorTestReconciler) reconcile() reconcile.Result {
	ctx := context.Background()
	xjoinIndexValidatorReconciler := x.newXJoinIndexValidatorReconciler()
	indexValidatorLookup := types.NamespacedName{Name: x.GetName(), Namespace: x.Namespace}
	result, err := xjoinIndexValidatorReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: indexValidatorLookup})
	checkError(err)
	return result
}

func (x *XJoinIndexValidatorTestReconciler) registerCreateMocks() {
	//avro schema mocks
	schema := fmt.Sprintf(`{"type":"record","name":"Value","namespace":"xjoinindexpipeline.%s","fields":[{"name":"%s","type":{"type":"record","name":"xjoindatasourcepipeline.%s.Value","fields":[]}}}`,
		x.Name, x.Name, x.Name)
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/1",
		httpmock.NewStringResponder(
			200, fmt.Sprintf(`{"id": 1, "subject": "xjoindatasourcepipeline.hosts.1674571335703357092-value", "version": 1, "schema": "%s", "references": "[]"}`, schema)))

	//
	//httpmock.RegisterResponder(
	//	"POST",
	//	"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.Name+".1234-value/versions",
	//	httpmock.NewStringResponder(200, `{"createdBy":"","createdOn":"2022-07-27T17:28:11+0000","modifiedBy":"","modifiedOn":"2022-07-27T17:28:11+0000","id":1,"version":1,"type":"AVRO","globalId":1,"state":"ENABLED","groupId":"null","contentId":1,"references":[]}`))
	//
	//httpmock.RegisterResponder(
	//	"GET",
	//	"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.Name+".1234-value/versions/latest",
	//	httpmock.NewStringResponder(200, `{"schema":"{\"name\":\"Value\",\"namespace\":\"xjoinindexpipelinepipeline.`+x.Name+`\"}","schemaType":"AVRO","references":[]}`))
}

func (x *XJoinIndexValidatorTestReconciler) registerRunningMocks() {
	schema := fmt.Sprintf(`{"type":"record","name":"Value","namespace":"xjoinindexpipeline.%s","fields":[{"name":"%s","type":{"type":"record","name":"xjoindatasourcepipeline.%s.Value","fields":[]}}}`,
		x.Name, x.Name, x.Name)
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/1",
		httpmock.NewStringResponder(
			200, fmt.Sprintf(`{"id": 1, "subject": "xjoindatasourcepipeline.hosts.1674571335703357092-value", "version": 1, "schema": "%s", "references": "[]"}`, schema)))
}

func (x *XJoinIndexValidatorTestReconciler) registerSuccessMocks() {
	schema := fmt.Sprintf(`{"type":"record","name":"Value","namespace":"xjoinindexpipeline.%s","fields":[{"name":"%s","type":{"type":"record","name":"xjoindatasourcepipeline.%s.Value","fields":[]}}}`,
		x.Name, x.Name, x.Name)
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/1",
		httpmock.NewStringResponder(
			200, fmt.Sprintf(`{"id": 1, "subject": "xjoindatasourcepipeline.hosts.1674571335703357092-value", "version": 1, "schema": "%s", "references": "[]"}`, schema)))
}

func (x *XJoinIndexValidatorTestReconciler) registerFailureMocks() {
	schema := fmt.Sprintf(`{"type":"record","name":"Value","namespace":"xjoinindexpipeline.%s","fields":[{"name":"%s","type":{"type":"record","name":"xjoindatasourcepipeline.%s.Value","fields":[]}}}`,
		x.Name, x.Name, x.Name)
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects/xjoinindexpipeline."+x.GetName()+"-value/versions/1",
		httpmock.NewStringResponder(
			200, fmt.Sprintf(`{"id": 1, "subject": "xjoindatasourcepipeline.hosts.1674571335703357092-value", "version": 1, "schema": "%s", "references": "[]"}`, schema)))
}
