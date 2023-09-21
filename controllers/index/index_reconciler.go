package index

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/common/labels"
	"github.com/redhatinsights/xjoin-operator/controllers/components"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcileMethods struct {
	iteration XJoinIndexIteration
	gvk       schema.GroupVersionKind
	log       logger.Log
	isTest    bool
}

func NewReconcileMethods(iteration XJoinIndexIteration, gvk schema.GroupVersionKind) *ReconcileMethods {
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
	d.iteration.Events.Normal("Removed", "Index was removed, running finalizer to cleanup child resources.")
	err = d.iteration.Finalize()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) New(version string) (err error) {
	d.iteration.Events.Normal("New", "Creating new IndexPipeline to initialize data sync process")
	err = d.iteration.CreateIndexPipeline(d.iteration.GetInstance().Name, version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) InitialSync() (err error) {
	d.iteration.Events.Normal("InitialSync", "Waiting for data to become in sync for the first time")
	err = d.iteration.ReconcileChildren()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) Valid() (err error) {
	d.iteration.Events.Normal("Valid", "Data is in sync")
	err = d.iteration.ReconcileChildren()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) RefreshFailed() (err error) {
	d.iteration.Events.Normal("RefreshFailed", "Refreshing pipeline failed, will try again.")
	err = d.iteration.DeleteIndexPipeline(d.iteration.GetInstance().Name, d.iteration.GetInstance().Status.RefreshingVersion)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) StartRefreshing(version string) (err error) {
	metrics.IndexRefreshing(d.iteration.GetInstance().GetName())
	d.iteration.Events.Normal("StartRefreshing", "Starting the refresh process by creating a new IndexPipeline")
	err = d.iteration.CreateIndexPipeline(d.iteration.GetInstance().Name, version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) Refreshing() (err error) {
	d.iteration.Events.Normal("Refreshing", "Refresh is in progress, waiting for the refreshing pipeline's data to be in sync")
	err = d.iteration.ReconcileChildren()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) RefreshComplete() (err error) {
	d.iteration.Events.Normal("RefreshComplete", "Refreshing pipeline is now in sync. Switching the refreshing pipeline to be active.")

	//update the status of the refreshing IndexPipeline to be active so the gateway starts using it
	refreshingPipeline := &v1alpha1.XJoinIndexPipeline{}
	indexPipelineLookup := types.NamespacedName{
		Namespace: d.iteration.GetInstance().GetNamespace(),
		Name:      d.iteration.GetInstance().GetName() + "." + d.iteration.GetInstance().Status.RefreshingVersion,
	}
	err = d.iteration.Client.Get(d.iteration.Context, indexPipelineLookup, refreshingPipeline)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	refreshingPipeline.Status.Active = true

	err = d.iteration.Client.Status().Update(d.iteration.Context, refreshingPipeline)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	//remove the invalid IndexPipeline
	if d.iteration.GetInstance().Status.ActiveVersion != "" {
		err = d.iteration.DeleteIndexPipeline(d.iteration.GetInstance().Name, d.iteration.GetInstance().Status.ActiveVersion)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return
}

func (d *ReconcileMethods) ScrubPipelines(validVersions []string) (err error) {
	existingIndexPipelines := &v1alpha1.XJoinIndexPipelineList{}
	labelsMatch := client.MatchingLabels{}
	labelsMatch[labels.ComponentName] = d.iteration.GetInstance().GetName()
	err = d.iteration.Client.List(
		d.iteration.Context, existingIndexPipelines, client.InNamespace(d.iteration.GetInstance().Namespace))
	if err != nil {
		return errors.Wrap(err, 0)
	}

	for _, pipeline := range existingIndexPipelines.Items {
		if !utils.ContainsString(validVersions, pipeline.Spec.Version) {
			err = d.iteration.DeleteIndexPipeline(pipeline.Spec.Name, pipeline.Spec.Version)
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

	genericElasticsearch, err := elasticsearch.NewGenericElasticsearch(elasticsearch.GenericElasticSearchParameters{
		Url:        d.iteration.Parameters.ElasticSearchURL.String(),
		Username:   d.iteration.Parameters.ElasticSearchUsername.String(),
		Password:   d.iteration.Parameters.ElasticSearchPassword.String(),
		Parameters: config.ParametersToMap(d.iteration.Parameters),
		Context:    d.iteration.Context,
	})
	if err != nil {
		return append(errs, errors.Wrap(err, 0))
	}

	schemaRegistryConnectionParams := schemaregistry.ConnectionParams{
		Protocol: d.iteration.Parameters.SchemaRegistryProtocol.String(),
		Hostname: d.iteration.Parameters.SchemaRegistryHost.String(),
		Port:     d.iteration.Parameters.SchemaRegistryPort.String(),
	}
	registryConfluentClient := schemaregistry.NewSchemaRegistryConfluentClient(schemaRegistryConnectionParams)
	registryConfluentClient.Init()
	registryRestClient := schemaregistry.NewSchemaRegistryRestClient(
		schemaRegistryConnectionParams, d.iteration.GetInstance().Namespace)

	custodian := components.NewCustodian(
		common.IndexPipelineGVK.Kind, d.iteration.GetInstance().Name, validVersions, d.iteration.Events, d.log)
	custodian.AddComponent(&components.ElasticsearchPipeline{
		GenericElasticsearch: *genericElasticsearch,
	})
	custodian.AddComponent(&components.ElasticsearchIndex{
		GenericElasticsearch: *genericElasticsearch,
	})

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
	custodian.AddComponent(&components.ElasticsearchConnector{
		KafkaClient: kafkaClient,
		Namespace:   d.iteration.GetInstance().Namespace,
	})
	custodian.AddComponent(components.NewAvroSchema(components.AvroSchemaParameters{
		Registry:    registryConfluentClient,
		KafkaClient: kafkaClient,
	}))
	custodian.AddComponent(components.NewGraphQLSchema(components.GraphQLSchemaParameters{
		Registry: registryRestClient,
	}))
	custodian.AddComponent(&components.XJoinCore{
		Client:    d.iteration.Client,
		Context:   d.iteration.Context,
		Namespace: d.iteration.GetInstance().Namespace,
	})
	custodian.AddComponent(&components.XJoinAPISubGraph{
		Client:    d.iteration.Client,
		Context:   d.iteration.Context,
		Registry:  registryConfluentClient,
		Namespace: d.iteration.GetInstance().Namespace,
	})
	custodian.AddComponent(&components.ValidationPod{
		Client:    d.iteration.Client,
		Context:   d.iteration.Context,
		Namespace: d.iteration.GetInstance().Namespace,
	})

	d.log.Debug("Scrubbing index", "ValidVersion", validVersions)

	return custodian.Scrub()
}
