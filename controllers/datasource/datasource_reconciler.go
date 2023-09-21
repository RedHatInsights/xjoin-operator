package datasource

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/common/labels"
	"github.com/redhatinsights/xjoin-operator/controllers/components"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcileMethods struct {
	iteration XJoinDataSourceIteration
	gvk       schema.GroupVersionKind
	log       logger.Log
	isTest    bool
}

func NewReconcileMethods(iteration XJoinDataSourceIteration, gvk schema.GroupVersionKind) *ReconcileMethods {
	return &ReconcileMethods{
		iteration: iteration,
		gvk:       gvk,
	}
}

func (d *ReconcileMethods) SetIsTest(isTest bool) {
	d.isTest = isTest
}

func (d *ReconcileMethods) SetLogger(log logger.Log) {
	d.log = log
}

func (d *ReconcileMethods) Removed() (err error) {
	meta.SetStatusCondition(&d.iteration.GetInstance().Status.Conditions, metav1.Condition{
		Type:   common.ValidConditionType,
		Status: metav1.ConditionFalse,
		Reason: common.RemovedReason,
	})

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
	metrics.DatasourceRefreshing(d.iteration.GetInstance().GetName())
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

func (d *ReconcileMethods) RefreshFailed() (err error) {
	d.iteration.Events.Normal("RefreshFailed", "Refreshing pipeline failed, will try again.")
	err = d.iteration.DeleteDataSourcePipeline(d.iteration.GetInstance().Name, d.iteration.GetInstance().Status.RefreshingVersion)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) ScrubPipelines(validVersions []string) (err error) {
	existingDatasourcePipelines := &v1alpha1.XJoinDataSourcePipelineList{}
	labelsMatch := client.MatchingLabels{}
	labelsMatch[labels.ComponentName] = d.iteration.GetInstance().GetName()
	err = d.iteration.Client.List(
		d.iteration.Context, existingDatasourcePipelines, client.InNamespace(d.iteration.GetInstance().Namespace))
	if err != nil {
		return errors.Wrap(err, 0)
	}

	var existingPipelineVersions []string
	for _, pipeline := range existingDatasourcePipelines.Items {
		existingPipelineVersions = append(existingPipelineVersions, pipeline.Spec.Version)
	}

	d.log.Debug("Installed pipeline versions during scrub", "versions", existingPipelineVersions)

	for _, pipelineVersion := range existingPipelineVersions {
		if !utils.ContainsString(validVersions, pipelineVersion) {
			err = d.iteration.DeleteDataSourcePipeline(d.iteration.GetInstance().GetName(), pipelineVersion)
			if err != nil {
				return errors.Wrap(err, 0)
			}
		}
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

	err := d.ScrubPipelines(validVersions)
	if err != nil {
		return append(errs, errors.Wrap(err, 0))
	}

	kafkaClient := kafka.GenericKafka{
		Context:          d.iteration.Context,
		Client:           d.iteration.Client,
		KafkaNamespace:   d.iteration.Parameters.KafkaClusterNamespace.String(),
		KafkaCluster:     d.iteration.Parameters.KafkaCluster.String(),
		ConnectNamespace: d.iteration.Parameters.ConnectClusterNamespace.String(),
		ConnectCluster:   d.iteration.Parameters.ConnectCluster.String(),
		Log:              d.log,
	}

	registry := schemaregistry.NewSchemaRegistryConfluentClient(
		schemaregistry.ConnectionParams{
			Protocol: d.iteration.Parameters.SchemaRegistryProtocol.String(),
			Hostname: d.iteration.Parameters.SchemaRegistryHost.String(),
			Port:     d.iteration.Parameters.SchemaRegistryPort.String(),
		})
	registry.Init()

	custodian := components.NewCustodian(
		common.DataSourcePipelineGVK.Kind, d.iteration.GetInstance().Name, validVersions, d.iteration.Events, d.log)
	custodian.AddComponent(components.NewAvroSchema(components.AvroSchemaParameters{
		Registry:    registry,
		KafkaClient: kafkaClient,
	}))

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
		KafkaClient: kafkaClient,
	})

	custodian.AddComponent(&components.DebeziumConnector{
		KafkaClient: kafkaClient,
		Namespace:   d.iteration.GetInstance().Namespace,
	})

	db := database.NewDatabase(database.DBParams{
		User:        d.iteration.Parameters.DatabaseUsername.String(),
		Password:    d.iteration.Parameters.DatabasePassword.String(),
		Host:        d.iteration.Parameters.DatabaseHostname.String(),
		Name:        d.iteration.Parameters.DatabaseName.String(),
		Port:        d.iteration.Parameters.DatabasePort.String(),
		SSLMode:     d.iteration.Parameters.DatabaseSSLMode.String(),
		SSLRootCert: d.iteration.Parameters.DatabaseSSLRootCert.String(),
		IsTest:      d.isTest,
	})
	err = db.Connect()
	if err != nil {
		return append(errs, errors.Wrap(err, 0))
	}
	defer db.Close()

	custodian.AddComponent(&components.ReplicationSlot{
		Namespace: d.iteration.GetInstance().Namespace,
		Database:  db,
	})

	d.log.Debug("Scrubbing DataSource", "ValidVersion", validVersions)

	return custodian.Scrub()
}
