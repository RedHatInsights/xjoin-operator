package test

import (
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"

	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

var Client client.Client

var testEnv *envtest.Environment
var cfg *rest.Config

func GetRootDir() string {
	_, b, _, _ := runtime.Caller(0)
	d := path.Join(path.Dir(b))
	return filepath.Dir(d)
}

func ForwardPorts() {
	cmd := exec.Command(GetRootDir() + "/dev/forward-ports-clowder.sh")
	err := cmd.Run()
	Expect(err).ToNot(HaveOccurred())
	time.Sleep(time.Second * 3)
}

/* Sets up Before/After hooks that initialize testEnv.
 * Registers CRDs.
 * Registers Ginkgo Handler.
 */
func Setup(t *testing.T, suiteName string) {

	var _ = BeforeSuite(func(done Done) {
		ForwardPorts()

		cmd := exec.Command(GetRootDir() + "/dev/cleanup.projects.sh")
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

		close(done)
	}, 60)

	AfterSuite(func() {
		By("tearing down the test environment")
		err := testEnv.Stop()
		Expect(err).ToNot(HaveOccurred())
	})

	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, suiteName, []Reporter{printer.NewlineReporter{}})
}
