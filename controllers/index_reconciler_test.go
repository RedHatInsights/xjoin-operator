package controllers_test

import (
	"context"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type IndexTestReconciler struct {
	Namespace            string
	Name                 string
	K8sClient            client.Client
	AvroSchemaFileName   string
	CustomSubgraphImages []v1alpha1.CustomSubgraphImage
}

func (i *IndexTestReconciler) ReconcileNew() v1alpha1.XJoinIndex {
	i.registerNewMocks()
	i.createValidIndex()
	result := i.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 30000000000}))

	createdIndex := &v1alpha1.XJoinIndex{}
	indexLookupKey := types.NamespacedName{Name: i.Name, Namespace: i.Namespace}

	Eventually(func() bool {
		err := i.K8sClient.Get(context.Background(), indexLookupKey, createdIndex)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
	Expect(createdIndex.Status.ActiveVersion).To(Equal(""))
	Expect(createdIndex.Status.ActiveVersionState.Result).To(Equal(""))
	Expect(createdIndex.Status.RefreshingVersion).ToNot(Equal(""))
	Expect(createdIndex.Status.RefreshingVersionState.Result).To(Equal(common.New))
	Expect(createdIndex.Status.SpecHash).ToNot(Equal(""))
	Expect(createdIndex.Finalizers).To(HaveLen(1))
	Expect(createdIndex.Finalizers).To(ContainElement("finalizer.xjoin.index.cloud.redhat.com"))

	info := httpmock.GetCallCountInfo()
	count := info["GET http://apicurio:1080/apis/ccompat/v6/subjects"]
	Expect(count).To(Equal(1))

	return *createdIndex
}

func (i *IndexTestReconciler) ReconcileDelete() {
	i.registerDeleteMocks()
	result := i.reconcile()
	Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 0}))

	indexList := v1alpha1.XJoinIndexList{}
	err := i.K8sClient.List(context.Background(), &indexList, client.InNamespace(i.Namespace))
	checkError(err)
	Expect(indexList.Items).To(HaveLen(0))
}

func (i *IndexTestReconciler) GetIndex() v1alpha1.XJoinIndex {
	index := &v1alpha1.XJoinIndex{}
	indexLookupKey := types.NamespacedName{Name: i.Name, Namespace: i.Namespace}
	Eventually(func() bool {
		err := i.K8sClient.Get(context.Background(), indexLookupKey, index)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())
	return *index
}

func (i *IndexTestReconciler) newXJoinIndexReconciler() *controllers.XJoinIndexReconciler {
	return controllers.NewXJoinIndexReconciler(
		i.K8sClient,
		scheme.Scheme,
		testLogger,
		record.NewFakeRecorder(100),
		i.Namespace,
		true)
}

func (i *IndexTestReconciler) createValidIndex() {
	ctx := context.Background()

	var avroSchemaFilename string
	if i.AvroSchemaFileName == "" {
		avroSchemaFilename = "xjoinindex"
	} else {
		avroSchemaFilename = i.AvroSchemaFileName
	}

	indexAvroSchema, err := os.ReadFile("./test/data/avro/" + avroSchemaFilename + ".json")
	checkError(err)

	indexSpec := v1alpha1.XJoinIndexSpec{
		AvroSchema: string(indexAvroSchema),
		Pause:      false,
	}

	if i.CustomSubgraphImages != nil {
		indexSpec.CustomSubgraphImages = i.CustomSubgraphImages
	}

	index := &v1alpha1.XJoinIndex{
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.Name,
			Namespace: i.Namespace,
		},
		Spec: indexSpec,
		TypeMeta: metav1.TypeMeta{
			APIVersion: "xjoin.cloud.redhat.com/v1alpha1",
			Kind:       "XJoinIndex",
		},
	}

	Expect(i.K8sClient.Create(ctx, index)).Should(Succeed())

	//validate index spec is created correctly
	indexLookupKey := types.NamespacedName{Name: i.Name, Namespace: i.Namespace}
	createdIndex := &v1alpha1.XJoinIndex{}

	Eventually(func() bool {
		err := i.K8sClient.Get(ctx, indexLookupKey, createdIndex)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
	Expect(createdIndex.Spec.Pause).Should(Equal(false))
	Expect(createdIndex.Spec.AvroSchema).Should(Equal(string(indexAvroSchema)))

	if i.CustomSubgraphImages != nil {
		Expect(createdIndex.Spec.CustomSubgraphImages).Should(Equal(i.CustomSubgraphImages))
	}
}

func (i *IndexTestReconciler) reconcile() reconcile.Result {
	xjoinIndexReconciler := i.newXJoinIndexReconciler()
	indexLookupKey := types.NamespacedName{Name: i.Name, Namespace: i.Namespace}
	result, err := xjoinIndexReconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: indexLookupKey})
	checkError(err)
	return result
}

func (i *IndexTestReconciler) ReconcileUpdated() v1alpha1.XJoinIndex {
	i.registerUpdatedMocks()
	i.reconcile()
	return i.GetIndex()
}

func (i *IndexTestReconciler) registerUpdatedMocks() {
	i.registerNewMocks()
}

func (i *IndexTestReconciler) registerNewMocks() {
	httpmock.Reset()
	httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/ccompat/v6/subjects",
		httpmock.NewStringResponder(200, `[]`))

	responder, err := httpmock.NewJsonResponder(200, httpmock.File("./test/data/apicurio/empty-response.json"))
	checkError(err)
	httpmock.RegisterResponder(
		"GET",
		"http://apicurio:1080/apis/registry/v2/search/artifacts?limit=500&labels=graphql",
		responder)

	httpmock.RegisterResponder(
		"GET",
		"http://localhost:9200/_ingest/pipeline/xjoinindex."+i.Name+"%2A",
		httpmock.NewStringResponder(404, "{}"))

	httpmock.RegisterResponder(
		"GET",
		"http://localhost:9200/_cat/indices/xjoinindex."+i.Name+".%2A?format=JSON&h=index",
		httpmock.NewStringResponder(200, "[]"))
}

func (i *IndexTestReconciler) registerDeleteMocks() {
	i.registerNewMocks()
}
