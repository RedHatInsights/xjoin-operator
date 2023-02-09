package test

import (
	"fmt"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var Client client.Client

var testEnv *envtest.Environment
var cfg *rest.Config

func GetRootDir() string {
	_, b, _, _ := runtime.Caller(0)
	d := path.Join(path.Dir(b))
	return filepath.Dir(d)
}

func UniqueNamespace(prefix string) (namespace string, err error) {
	namespace = fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = Client.Create(ctx, ns)
	return
}

func StrToBool(str string) (bool, error) {
	return strconv.ParseBool(str)
}

func StrToInt64(str string) (int64, error) {
	return strconv.ParseInt(str, 10, 64)
}

func ForwardPorts() {
	cmd := exec.Command(GetRootDir() + "/../dev/forward-ports-clowder.sh")
	err := cmd.Run()
	Expect(err).ToNot(HaveOccurred())
	time.Sleep(time.Second * 3)
}

/* Sets up Before/After hooks that initialize testEnv.
 * Registers CRDs.
 * Registers Ginkgo Handler.
 */
var _ = BeforeSuite(func() {
	ForwardPorts()

	cmd := exec.Command(GetRootDir() + "/../dev/cleanup.projects.sh")
	err := cmd.Run()
	Expect(err).ToNot(HaveOccurred())

	useExistingCluster := true

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:  []string{filepath.Join(GetRootDir(), "config", "crd", "bases"), filepath.Join(GetRootDir(), "dev", "cluster-operator", "crd")},
		UseExistingCluster: &useExistingCluster,
	}

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = xjoin.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	Client, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(Client).ToNot(BeNil())

	_, err = kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())

	//make sure the test environment is clean
	dbClient := database.NewDatabase(database.DBParams{
		Host:     "host-inventory-db.test.svc",
		Port:     "5432",
		User:     "insights",
		Password: "insights",
		Name:     "test",
		SSLMode:  "disable",
	})

	err = dbClient.Connect()
	Expect(err).ToNot(HaveOccurred())

	err = dbClient.RemoveReplicationSlotsForPrefix("xjointest")
	Expect(err).ToNot(HaveOccurred())
	err = dbClient.RemoveReplicationSlotsForPrefix("xjointestupdated")
	Expect(err).ToNot(HaveOccurred())
	err = dbClient.Close()
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}
