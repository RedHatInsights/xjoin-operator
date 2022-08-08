package kafka

import (
	"context"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = logger.NewLogger("kafka")

type Kafka struct {
	GenericKafka
	Namespace     string
	OwnerScheme   *runtime.Scheme
	Client        client.Client
	Parameters    config.Parameters
	ParametersMap map[string]interface{}
	Recorder      record.EventRecorder
	Test          bool
}

type GenericKafka struct {
	Context          context.Context
	Client           client.Client
	KafkaNamespace   string
	KafkaCluster     string
	ConnectNamespace string
	ConnectCluster   string
	Test             bool
}

type Topics interface {
	TopicName(pipelineVersion string) string
	CreateTopic(pipelineVersion string, dryRun bool) error
	DeleteTopicByPipelineVersion(pipelineVersion string) error
	DeleteAllTopics() error
	ListTopicNamesForPipelineVersion(pipelineVersion string) ([]string, error)
	CheckDeviation(string) (error, error)
}

type StrimziTopics struct {
	Kafka Kafka
}

type ManagedTopics struct {
	Kafka Kafka
}

type Connectors interface {
	newESConnectorResource(pipelineVersion string) (*unstructured.Unstructured, error)
	newDebeziumConnectorResource(pipelineVersion string) (*unstructured.Unstructured, error)
	newConnectorResource(
		name string,
		class string,
		connectorConfig map[string]interface{},
		connectorTemplate string) (*unstructured.Unstructured, error)
	GetTaskStatus(connectorName string, taskId float64) (map[string]interface{}, error)
	ListConnectorTasks(connectorName string) ([]map[string]interface{}, error)
	RestartTaskForConnector(connectorName string, taskId float64) error
	verifyTaskIsRunning(task map[string]interface{}, connectorName string) (bool, error)
	RestartConnector(connectorName string) error
	CheckIfConnectIsResponding() (bool, error)
	ListConnectorsREST(prefix string) ([]string, error)
	DeleteConnectorsForPipelineVersion(pipelineVersion string) error
	ListConnectorNamesForPipelineVersion(pipelineVersion string) ([]string, error)
	DeleteAllConnectors(resourceNamePrefix string) error
	GetConnectorStatus(connectorName string) (map[string]interface{}, error)
	IsFailed(connectorName string) (bool, error)
	CreateESConnector(
		pipelineVersion string,
		dryRun bool) (*unstructured.Unstructured, error)
	CreateDebeziumConnector(
		pipelineVersion string,
		dryRun bool) (*unstructured.Unstructured, error)
	DebeziumConnectorName(pipelineVersion string) string
	ESConnectorName(pipelineVersion string) string
	PauseElasticSearchConnector(pipelineVersion string) error
	ResumeElasticSearchConnector(pipelineVersion string) error
	setElasticSearchConnectorPause(pipelineVersion string, pause bool) error
	CreateDryConnectorByType(conType string, version string) (*unstructured.Unstructured, error)
	updateConnectDepReplicas(newReplicas int64) (currentReplicas int64, err error)
	RestartConnect() error
}

type StrimziConnectors struct {
	Kafka  Kafka
	Topics Topics
}
