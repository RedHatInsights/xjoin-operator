package controllers

import (
	"context"
	"github.com/go-errors/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"path/filepath"
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
var namespace string

var testLogger = logf.Log.WithName("test")

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

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	namespace = "default"

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
