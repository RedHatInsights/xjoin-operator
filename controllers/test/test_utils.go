package test

import (
	"context"
	. "github.com/onsi/gomega"
	. "github.com/redhatinsights/xjoin-operator/controllers"
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"github.com/redhatinsights/xjoin-operator/test"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"os/exec"
	"reflect"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
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
		SSL:      "disable",
	})

	err = i.DbClient.Connect()
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

	projects := &unstructured.UnstructuredList{}
	projects.SetKind("Namespace")
	projects.SetAPIVersion("v1")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err = i.XJoinReconciler.Client.List(ctx, projects)
	Expect(err).ToNot(HaveOccurred())

	//remove finalizers from leftover pipelines so the project can be deleted
	for _, p := range i.Pipelines {
		pipelines, err :=
			utils.FetchXJoinPipelinesByNamespacedName(test.Client, p.Name, p.Namespace)
		Expect(err).ToNot(HaveOccurred())
		if len(pipelines.Items) != 0 {
			pipeline := pipelines.Items[0]
			_ = i.KafkaClient.DeleteConnectorsForPipelineVersion(pipeline.Status.PipelineVersion)
			_ = i.KafkaClient.DeleteTopicByPipelineVersion(pipeline.Status.PipelineVersion)
			_ = i.DbClient.RemoveReplicationSlotsForPipelineVersion(pipeline.Status.PipelineVersion)
			_ = i.EsClient.DeleteIndex(pipeline.Status.PipelineVersion)
			_ = i.EsClient.DeleteESPipelineByVersion(pipeline.Status.PipelineVersion)

			_ = i.KafkaClient.DeleteConnector(pipeline.Status.ActiveDebeziumConnectorName)
			_ = i.KafkaClient.DeleteConnector(pipeline.Status.ActiveESConnectorName)
			_ = i.KafkaClient.DeleteTopic(pipeline.Status.ActiveTopicName)
			_ = i.DbClient.RemoveReplicationSlot(pipeline.Status.ActiveReplicationSlotName)
			_ = i.EsClient.DeleteIndexByFullName(pipeline.Status.ActiveIndexName)
			_ = i.EsClient.DeleteESPipelineByFullName(pipeline.Status.ActiveESPipelineName)

			i.DeleteAllHosts()
			if pipeline.DeletionTimestamp == nil {
				pipeline.ObjectMeta.Finalizers = nil
				err = i.XJoinReconciler.Client.Update(ctx, &pipeline)
				Expect(err).ToNot(HaveOccurred())
			}
		}
	}

	//delete leftover projects
	for _, p := range projects.Items {
		if strings.Index(p.GetName(), ResourceNamePrefix) == 0 && p.GetDeletionTimestamp() == nil {
			project := &unstructured.Unstructured{}
			project.SetName(p.GetName())
			project.SetNamespace(p.GetNamespace())
			project.SetGroupVersionKind(p.GroupVersionKind())
			err = i.XJoinReconciler.Client.Delete(ctx, project)
			Expect(err).ToNot(HaveOccurred())
			err = i.KafkaClient.DeleteConnectorsForPipelineVersion("1")
		}
	}

	i.DbClient.Close()

	cmd := exec.Command(test.GetRootDir() + "/dev/cleanup.projects.sh")
	err = cmd.Run()
	Expect(err).ToNot(HaveOccurred())
}
