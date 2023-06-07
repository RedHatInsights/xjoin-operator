package controllers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/go-errors/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	strimziApi "github.com/RedHatInsights/strimzi-client-go/apis/kafka.strimzi.io/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	xjoinApi "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

var K8sGetTimeout = 3 * time.Second
var K8sGetInterval = 100 * time.Millisecond

var testLogger = logf.Log.WithName("test")

// this is used to output stack traces when an error occurs
func checkError(err error) {
	if err != nil {
		testLogger.Error(errors.Wrap(err, 0), "test failure")
	}
	Expect(err).ToNot(HaveOccurred())
}

func k8sGet(key client.ObjectKey, obj client.Object) {
	ctx := context.Background()
	Eventually(func() bool {
		err := k8sClient.Get(ctx, key, obj)
		return err == nil
	}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t,
		"Controller Suite")
}

func NewNamespace() (string, error) {
	name := "test" + strconv.FormatInt(time.Now().UnixNano(), 10)
	namespace := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := k8sClient.Create(context.Background(), &namespace)
	if err != nil {
		return "", err
	}

	configMap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xjoin-generic",
			Namespace: name,
		},
		Data: map[string]string{
			"kafka.cluster.namespace":   name,
			"connect.cluster.namespace": name,
			"schemaregistry.port":       "1080",
			"schemaregistry.host":       "apicurio",
		},
	}
	err = k8sClient.Create(context.Background(), &configMap)
	if err != nil {
		return "", err
	}

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xjoin-elasticsearch",
			Namespace: name,
		},
		Type: "opaque",
		StringData: map[string]string{
			"endpoint": "http://localhost:9200",
			"password": "xjoin1337",
			"username": "xjoin",
		},
	}
	err = k8sClient.Create(context.Background(), &secret)
	checkError(err)

	return name, nil
}

func LoadExpectedKafkaResourceConfig(filename string) *bytes.Buffer {
	file, err := os.ReadFile(filename)
	checkError(err)
	buffer := bytes.NewBuffer([]byte{})
	err = json.Compact(buffer, file)
	checkError(err)
	return buffer
}

type IndexPipelineTestResources struct {
	IndexReconciler         IndexTestReconciler
	IndexPipelineReconciler XJoinIndexPipelineTestReconciler
	DatasourceReconciler    DatasourceTestReconciler
	IndexPipeline           xjoinApi.XJoinIndexPipeline
	Index                   xjoinApi.XJoinIndex
	DataSource              xjoinApi.XJoinDataSource
}

type UpdatedMocksParams struct {
	GraphQLSchemaExistingState string
	GraphQLSchemaNewState      string
}

func CreateValidIndexPipeline(namespace string, customSubgraphImages []xjoinApi.CustomSubgraphImage) IndexPipelineTestResources {
	indexReconciler := IndexTestReconciler{
		Namespace:            namespace,
		Name:                 "test-index",
		K8sClient:            k8sClient,
		AvroSchemaFileName:   "xjoinindex-with-referenced-field",
		CustomSubgraphImages: customSubgraphImages,
	}
	createdIndex := indexReconciler.ReconcileNew()

	//create a valid datasource
	dataSourceName := "testdatasource"
	datasourceReconciler := DatasourceTestReconciler{
		Namespace: namespace,
		Name:      dataSourceName,
		K8sClient: k8sClient,
	}
	datasourceReconciler.ReconcileNew()
	createdDataSource := datasourceReconciler.ReconcileValid()

	//reconcile the refreshing indexpipeline to be valid
	indexPipelineReconciler := XJoinIndexPipelineTestReconciler{
		Namespace:            namespace,
		Name:                 createdIndex.Name,
		Version:              createdIndex.Status.RefreshingVersion,
		ConfigFileName:       "xjoinindex-with-referenced-field",
		K8sClient:            k8sClient,
		CustomSubgraphImages: customSubgraphImages,
		DataSources: []DataSource{{
			Name:                     dataSourceName,
			Version:                  createdDataSource.Status.ActiveVersion,
			ApiCurioResponseFilename: "datasource-latest-version",
		}},
	}
	indexPipeline := indexPipelineReconciler.ReconcileUpdated(UpdatedMocksParams{
		GraphQLSchemaExistingState: "DISABLED",
		GraphQLSchemaNewState:      "DISABLED",
	})
	Expect(indexPipeline.Status.Active).To(Equal(false))

	//reconcile the index to flip the refreshing pipeline to be active
	indexReconciler.ReconcileUpdated()
	indexPipeline = indexPipelineReconciler.ReconcileUpdated(UpdatedMocksParams{
		GraphQLSchemaExistingState: "DISABLED",
		GraphQLSchemaNewState:      "ENABLED",
	})

	return IndexPipelineTestResources{
		IndexReconciler:         indexReconciler,
		IndexPipelineReconciler: indexPipelineReconciler,
		DatasourceReconciler:    datasourceReconciler,
		IndexPipeline:           indexPipeline,
		Index:                   createdIndex,
		DataSource:              createdDataSource,
	}
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("test", "data", "crd"),
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	myscheme, err := xjoinApi.SchemeBuilder.Build()
	Expect(err).NotTo(HaveOccurred())

	err = scheme.AddToScheme(myscheme)
	Expect(err).NotTo(HaveOccurred())

	//err = xjoinApi.AddToScheme(scheme.Scheme)
	//Expect(err).NotTo(HaveOccurred())
	//
	err = strimziApi.AddToScheme(myscheme)
	Expect(err).NotTo(HaveOccurred())

	gvk := schema.GroupVersionKind{Version: "v1beta2", Kind: "KafkaTopic", Group: "kafka.strimzi.io"}
	recognized := myscheme.Recognizes(gvk)
	Expect(recognized).To(Equal(true))

	//+kubebuilder:scaffold:scheme
	k8sClient, err = client.New(cfg, client.Options{Scheme: myscheme})

	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
