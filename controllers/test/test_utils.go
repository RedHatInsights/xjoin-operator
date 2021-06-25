package test

import (
	. "github.com/onsi/gomega"
	. "github.com/redhatinsights/xjoin-operator/controllers"
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/test"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"os/exec"
	"reflect"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

var log = logger.NewLogger("test_utils")

func newXJoinReconciler(namespace string, isTest bool) *XJoinPipelineReconciler {
	return NewXJoinReconciler(
		test.Client,
		scheme.Scheme,
		logf.Log.WithName("test"),
		record.NewFakeRecorder(10),
		namespace,
		isTest)
}

func newValidationReconciler(namespace string) *ValidationReconciler {
	return NewValidationReconciler(
		test.Client,
		scheme.Scheme,
		logf.Log.WithName("test-validation"),
		true,
		record.NewFakeRecorder(10),
		namespace,
		true)
}

func newKafkaConnectReconciler(namespace string, isTest bool) *KafkaConnectReconciler {
	return NewKafkaConnectReconciler(
		test.Client,
		scheme.Scheme,
		logf.Log.WithName("test-kafkaconnect"),
		record.NewFakeRecorder(10),
		namespace,
		isTest)
}

func parametersToMap(parameters Parameters) map[string]interface{} {
	configReflection := reflect.ValueOf(&parameters).Elem()
	parametersMap := make(map[string]interface{})

	for i := 0; i < configReflection.NumField(); i++ {
		param := configReflection.Field(i).Interface().(Parameter)
		parametersMap[configReflection.Type().Field(i).Name] = param.Value()
	}

	return parametersMap
}

func getParameters() (Parameters, map[string]interface{}) {
	options := viper.New()
	options.SetDefault("ElasticSearchURL", "http://xjoin-elasticsearch-es-http:9200")
	options.SetDefault("ElasticSearchUsername", "test")
	options.SetDefault("ElasticSearchPassword", "test1337")
	options.SetDefault("HBIDBHost", "inventory-db")
	options.SetDefault("HBIDBPort", "5432")
	options.SetDefault("HBIDBUser", "insights")
	options.SetDefault("HBIDBPassword", "insights")
	options.SetDefault("HBIDBName", "test")
	options.SetDefault("ResourceNamePrefix", ResourceNamePrefix)
	options.AutomaticEnv()

	xjoinConfiguration := NewXJoinConfiguration()
	err := xjoinConfiguration.ElasticSearchURL.SetValue(options.GetString("ElasticSearchURL"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.ElasticSearchUsername.SetValue(options.GetString("ElasticSearchUsername"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.ElasticSearchPassword.SetValue(options.GetString("ElasticSearchPassword"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.HBIDBHost.SetValue(options.GetString("HBIDBHost"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.HBIDBPort.SetValue(options.GetString("HBIDBPort"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.HBIDBUser.SetValue(options.GetString("HBIDBUser"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.HBIDBPassword.SetValue(options.GetString("HBIDBPassword"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.HBIDBName.SetValue(options.GetString("HBIDBName"))
	Expect(err).ToNot(HaveOccurred())
	err = xjoinConfiguration.ResourceNamePrefix.SetValue(options.GetString("ResourceNamePrefix"))
	Expect(err).ToNot(HaveOccurred())

	return xjoinConfiguration, parametersToMap(xjoinConfiguration)
}

func Before() *Iteration {
	i := NewTestIteration()

	i.NamespacedName = types.NamespacedName{
		Name:      "test-pipeline-01",
		Namespace: test.UniqueNamespace(ResourceNamePrefix),
	}

	log.Info("Namespace: " + i.NamespacedName.Namespace)

	i.XJoinReconciler = newXJoinReconciler(i.NamespacedName.Namespace, true)
	i.ValidationReconciler = newValidationReconciler(i.NamespacedName.Namespace)
	i.KafkaConnectReconciler = newKafkaConnectReconciler(i.NamespacedName.Namespace, true)

	i.Parameters, i.ParametersMap = getParameters()
	i.CreateDbSecret("host-inventory-db")
	i.CreateESSecret("xjoin-elasticsearch")

	es, err := elasticsearch.NewElasticSearch(
		"http://xjoin-elasticsearch-es-http:9200",
		"xjoin",
		"xjoin1337",
		ResourceNamePrefix,
		i.Parameters.ElasticSearchPipelineTemplate.String(),
		i.Parameters.ElasticSearchIndexTemplate.String(),
		i.ParametersMap)

	i.EsClient = es
	Expect(err).ToNot(HaveOccurred())

	i.KafkaClient = kafka.Kafka{
		Namespace:     i.NamespacedName.Namespace,
		Client:        i.XJoinReconciler.Client,
		Parameters:    i.Parameters,
		ParametersMap: i.ParametersMap,
	}

	i.DbClient = database.NewDatabase(database.DBParams{
		Host:     i.Parameters.HBIDBHost.String(),
		Port:     i.Parameters.HBIDBPort.String(),
		User:     "postgres",
		Password: "postgres",
		Name:     i.Parameters.HBIDBName.String(),
		SSLMode:  "disable",
	})

	//Sometimes during the test suite execution the port forward fails.
	//This will check each dependency and if one is not responding,
	//the port will be forwarded again
	for j := 0; j < 3; j++ {
		dependenciesAreResponding := i.CheckIfDependenciesAreResponding()

		if dependenciesAreResponding {
			break
		} else {
			//there is a slight lag between running oc port-forward and being able to access the service
			//which is why the sleep is here
			cmd := exec.Command(test.GetRootDir() + "/dev/forward-ports.sh")
			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred())
			time.Sleep(1 * time.Second)
		}
	}

	Expect(err).ToNot(HaveOccurred())
	i.DbClient.SetMaxConnections(1000)

	_, err = i.DbClient.ExecQuery("DELETE FROM hosts")
	Expect(err).ToNot(HaveOccurred())

	return i
}

func After(i *Iteration) {
	err := i.DbClient.Connect()
	Expect(err).ToNot(HaveOccurred())
	defer i.CloseDB()

	//Delete any leftover ES indices
	indices, err := i.EsClient.ListIndices()
	Expect(err).ToNot(HaveOccurred())
	for _, index := range indices {
		err = i.EsClient.DeleteIndexByFullName(index)
		Expect(err).ToNot(HaveOccurred())
	}

	cmd := exec.Command(test.GetRootDir() + "/dev/cleanup.projects.sh")
	err = cmd.Run()
	Expect(err).ToNot(HaveOccurred())
}
