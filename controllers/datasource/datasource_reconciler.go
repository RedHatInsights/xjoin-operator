package datasource

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/components"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ReconcileMethods struct {
	iteration XJoinDataSourceIteration
	gvk       schema.GroupVersionKind
	log       logger.Log
}

func NewReconcileMethods(iteration XJoinDataSourceIteration, gvk schema.GroupVersionKind) *ReconcileMethods {
	return &ReconcileMethods{
		iteration: iteration,
		gvk:       gvk,
	}
}

func (d *ReconcileMethods) SetLogger(log logger.Log) {
	d.log = log
}

func (d *ReconcileMethods) Removed() (err error) {
	d.iteration.Events.Normal("Removed", "Datasource was removed, running finalizer to cleanup child resources.")

	err = d.iteration.Finalize()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) New(version string) (err error) {
	d.iteration.Events.Normal("New", "Creating new DataSource pipeline to initialize data sync process")
	err = d.iteration.CreateDataSourcePipeline(d.iteration.GetInstance().Name, version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) InitialSync() (err error) {
	d.iteration.Events.Normal("InitialSync", "Waiting for data to become in sync for the first time")
	err = d.iteration.ReconcilePipelines()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) Valid() (err error) {
	d.iteration.Events.Normal("Valid", "Data is in sync")
	err = d.iteration.ReconcilePipelines()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) StartRefreshing(version string) (err error) {
	d.iteration.Events.Normal("StartRefreshing", "Starting the refresh process by creating a new DatasourcePipeline")
	err = d.iteration.CreateDataSourcePipeline(d.iteration.GetInstance().Name, version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) Refreshing() (err error) {
	d.iteration.Events.Normal("Refreshing", "Refresh is in progress, waiting for the refreshing pipeline's data to be in sync")
	err = d.iteration.ReconcilePipelines()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) RefreshComplete() (err error) {
	d.iteration.Events.Normal("RefreshComplete", "Refreshing pipeline is now in sync. Switching the refreshing pipeline to be active.")
	err = d.iteration.DeleteDataSourcePipeline(d.iteration.GetInstance().Name, d.iteration.GetInstance().Status.ActiveVersion)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) Scrub() (errs []error) {
	var validVersions []string
	if d.iteration.GetInstance().Status.ActiveVersion != "" {
		validVersions = append(validVersions, d.iteration.GetInstance().Status.ActiveVersion)
	}
	if d.iteration.GetInstance().Status.RefreshingVersion != "" {
		validVersions = append(validVersions, d.iteration.GetInstance().Status.RefreshingVersion)
	}

	kafkaClient := kafka.GenericKafka{
		Context:          d.iteration.Context,
		Client:           d.iteration.Client,
		KafkaNamespace:   d.iteration.Parameters.KafkaClusterNamespace.String(),
		KafkaCluster:     d.iteration.Parameters.KafkaCluster.String(),
		ConnectNamespace: d.iteration.Parameters.ConnectClusterNamespace.String(),
		ConnectCluster:   d.iteration.Parameters.ConnectCluster.String(),
	}

	registry := schemaregistry.NewSchemaRegistryConfluentClient(
		schemaregistry.ConnectionParams{
			Protocol: d.iteration.Parameters.SchemaRegistryProtocol.String(),
			Hostname: d.iteration.Parameters.SchemaRegistryHost.String(),
			Port:     d.iteration.Parameters.SchemaRegistryPort.String(),
		})
	registry.Init()

	custodian := components.NewCustodian(
		d.gvk.Kind, d.iteration.GetInstance().Name, validVersions, d.iteration.Events, d.log)
	custodian.AddComponent(components.NewAvroSchema(components.AvroSchemaParameters{Registry: registry}))

	kafkaTopics := kafka.StrimziTopics{
		TopicParameters: kafka.TopicParameters{
			Replicas:           d.iteration.Parameters.KafkaTopicReplicas.Int(),
			Partitions:         d.iteration.Parameters.KafkaTopicPartitions.Int(),
			CleanupPolicy:      d.iteration.Parameters.KafkaTopicCleanupPolicy.String(),
			MinCompactionLagMS: d.iteration.Parameters.KafkaTopicMinCompactionLagMS.String(),
			RetentionBytes:     d.iteration.Parameters.KafkaTopicRetentionBytes.String(),
			RetentionMS:        d.iteration.Parameters.KafkaTopicRetentionMS.String(),
			MessageBytes:       d.iteration.Parameters.KafkaTopicMessageBytes.String(),
			CreationTimeout:    d.iteration.Parameters.KafkaTopicCreationTimeout.Int(),
		},
		KafkaClusterNamespace: d.iteration.Parameters.KafkaClusterNamespace.String(),
		KafkaCluster:          d.iteration.Parameters.KafkaCluster.String(),
		Client:                d.iteration.Client,
		Test:                  d.iteration.Test,
		Context:               d.iteration.Context,
		//ResourceNamePrefix:  this is not needed for generic topics
	}
	custodian.AddComponent(&components.KafkaTopic{
		KafkaTopics: kafkaTopics,
	})

	custodian.AddComponent(&components.DebeziumConnector{
		KafkaClient: kafkaClient,
		Namespace:   d.iteration.GetInstance().Namespace,
	})

	d.log.Debug("Scrubbing DataSource", "ValidVersion", validVersions)

	return custodian.Scrub()
}
