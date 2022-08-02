package controllers

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
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

var testLogger = logf.Log.WithName("test")

var k8sGetTimeout = 3 * time.Second
var k8sGetInterval = 100 * time.Millisecond

//this is used to output stack traces when an error occurs
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
		if err != nil {
			return false
		}
		return true
	}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
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

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "test", "crd"),
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

}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
