package test

import (
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

func getParameters() (Parameters, map[string]interface{}, error) {
	options := viper.New()
	options.SetDefault("ElasticSearchURL", "http://xjoin-elasticsearch-es-http:9200")
	options.SetDefault("ElasticSearchUsername", "test")
	options.SetDefault("ElasticSearchPassword", "test1337")
	options.SetDefault("HBIDBHost", "host-inventory-db.test.svc")
	options.SetDefault("HBIDBPort", "5432")
	options.SetDefault("HBIDBUser", "insights")
	options.SetDefault("HBIDBPassword", "insights")
	options.SetDefault("HBIDBName", "test")
	options.SetDefault("ResourceNamePrefix", ResourceNamePrefix)
	options.SetDefault("ConnectClusterNamespace", "test")
	options.SetDefault("ConnectCluster", "connect")
	options.SetDefault("KafkaClusterNamespace", "test")
	options.SetDefault("KafkaCluster", "kafka")
	options.AutomaticEnv()

	xjoinConfiguration := NewXJoinConfiguration()
	err := xjoinConfiguration.ElasticSearchURL.SetValue(options.GetString("ElasticSearchURL"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.ElasticSearchUsername.SetValue(options.GetString("ElasticSearchUsername"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.ElasticSearchPassword.SetValue(options.GetString("ElasticSearchPassword"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.HBIDBHost.SetValue(options.GetString("HBIDBHost"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.HBIDBPort.SetValue(options.GetString("HBIDBPort"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.HBIDBUser.SetValue(options.GetString("HBIDBUser"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.HBIDBPassword.SetValue(options.GetString("HBIDBPassword"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.HBIDBName.SetValue(options.GetString("HBIDBName"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.ResourceNamePrefix.SetValue(options.GetString("ResourceNamePrefix"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.ConnectCluster.SetValue(options.GetString("ConnectCluster"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.ConnectClusterNamespace.SetValue(options.GetString("ConnectClusterNamespace"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.KafkaCluster.SetValue(options.GetString("KafkaCluster"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}
	err = xjoinConfiguration.KafkaClusterNamespace.SetValue(options.GetString("KafkaClusterNamespace"))
	if err != nil {
		return xjoinConfiguration, nil, err
	}

	return xjoinConfiguration, parametersToMap(xjoinConfiguration), nil
}

func Before() (*Iteration, error) {
	i := NewTestIteration()
	ns, err := test.UniqueNamespace(ResourceNamePrefix)
	if err != nil {
		return nil, err
	}

	i.NamespacedName = types.NamespacedName{
		Name:      "test-pipeline-01",
		Namespace: ns,
	}

	log.Info("Namespace: " + i.NamespacedName.Namespace)

	i.XJoinReconciler = newXJoinReconciler(i.NamespacedName.Namespace, true)
	i.ValidationReconciler = newValidationReconciler(i.NamespacedName.Namespace)
	i.KafkaConnectReconciler = newKafkaConnectReconciler(i.NamespacedName.Namespace, true)

	parameters, parametersMap, err := getParameters()
	if err != nil {
		return nil, err
	}
	i.Parameters = parameters
	i.ParametersMap = parametersMap

	err = i.CreateDbSecret("host-inventory-db")
	if err != nil {
		return nil, err
	}
	err = i.CreateESSecret("xjoin-elasticsearch")
	if err != nil {
		return nil, err
	}

	es, err := elasticsearch.NewElasticSearch(
		"http://xjoin-elasticsearch-es-http:9200",
		"xjoin",
		"xjoin1337",
		ResourceNamePrefix,
		i.Parameters.ElasticSearchPipelineTemplate.String(),
		i.Parameters.ElasticSearchIndexTemplate.String(),
		i.ParametersMap)

	if err != nil {
		return nil, err
	}

	i.EsClient = es

	i.KafkaClient = kafka.Kafka{
		Namespace:     i.NamespacedName.Namespace,
		Client:        i.XJoinReconciler.Client,
		Parameters:    i.Parameters,
		ParametersMap: i.ParametersMap,
	}

	i.DbClient = database.NewDatabase(database.DBParams{
		Host:     i.Parameters.HBIDBHost.String(),
		Port:     i.Parameters.HBIDBPort.String(),
		User:     "insights",
		Password: "insights",
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
			cmd := exec.Command(test.GetRootDir() + "/dev/forward-ports-clowder.sh")
			err := cmd.Run()
			if err != nil {
				return nil, err
			}
			time.Sleep(1 * time.Second)
		}
	}

	i.DbClient.SetMaxConnections(1000)

	err = i.DbClient.Connect()
	if err != nil {
		return nil, err
	}

	_, err = i.DbClient.ExecQuery("DELETE FROM hosts")
	if err != nil {
		return nil, err
	}

	return i, nil
}

func After(i *Iteration) error {
	err := i.DbClient.Connect()
	if err != nil {
		return err
	}
	defer i.CloseDB()

	//Delete any leftover ES indices
	indices, err := i.EsClient.ListIndices()
	if err != nil {
		return err
	}
	for _, index := range indices {
		err = i.EsClient.DeleteIndexByFullName(index)
		if err != nil {
			return err
		}
	}

	cmd := exec.Command(test.GetRootDir() + "/dev/cleanup.projects.sh")
	return cmd.Run()
}
