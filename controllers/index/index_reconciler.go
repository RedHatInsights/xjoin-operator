package index

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/components"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
)

type ReconcileMethods struct {
	iteration XJoinIndexIteration
}

func NewReconcileMethods(iteration XJoinIndexIteration) *ReconcileMethods {
	return &ReconcileMethods{
		iteration: iteration,
	}
}

func (d *ReconcileMethods) Removed() (err error) {
	err = d.iteration.Finalize()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) New(version string) (err error) {
	err = d.iteration.CreateIndexPipeline(d.iteration.GetInstance().Name, version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	err = d.iteration.CreateIndexValidator(d.iteration.GetInstance().Name, version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) InitialSync() (err error) {
	err = d.iteration.ReconcileChildren()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) Valid() (err error) {
	err = d.iteration.ReconcileChildren()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) StartRefreshing(version string) (err error) {
	err = d.iteration.CreateIndexPipeline(d.iteration.GetInstance().Name, version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	err = d.iteration.CreateIndexValidator(d.iteration.GetInstance().Name, version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) Refreshing() (err error) {
	err = d.iteration.ReconcileChildren()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) RefreshComplete() (err error) {
	err = d.iteration.DeleteIndexPipeline(d.iteration.GetInstance().Name, d.iteration.GetInstance().Status.ActiveVersion)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	err = d.iteration.DeleteIndexValidator(d.iteration.GetInstance().Name, d.iteration.GetInstance().Status.ActiveVersion)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) Scrub() (err error) {
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

	genericElasticsearch, err := elasticsearch.NewGenericElasticsearch(elasticsearch.GenericElasticSearchParameters{
		Url:        d.iteration.Parameters.ElasticSearchURL.String(),
		Username:   d.iteration.Parameters.ElasticSearchUsername.String(),
		Password:   d.iteration.Parameters.ElasticSearchPassword.String(),
		Parameters: config.ParametersToMap(d.iteration.Parameters),
		Context:    d.iteration.Context,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	schemaRegistryConnectionParams := schemaregistry.ConnectionParams{
		Protocol: d.iteration.Parameters.SchemaRegistryProtocol.String(),
		Hostname: d.iteration.Parameters.SchemaRegistryHost.String(),
		Port:     d.iteration.Parameters.SchemaRegistryPort.String(),
	}
	registryConfluentClient := schemaregistry.NewSchemaRegistryConfluentClient(schemaRegistryConnectionParams)
	registryConfluentClient.Init()
	registryRestClient := schemaregistry.NewSchemaRegistryRestClient(schemaRegistryConnectionParams)

	custodian := components.NewCustodian(
		d.iteration.GetInstance().Kind+"."+d.iteration.GetInstance().Name, validVersions)
	custodian.AddComponent(&components.ElasticsearchPipeline{
		GenericElasticsearch: *genericElasticsearch,
	})
	custodian.AddComponent(&components.ElasticsearchIndex{
		GenericElasticsearch: *genericElasticsearch,
	})
	custodian.AddComponent(&components.KafkaTopic{KafkaClient: kafkaClient})
	custodian.AddComponent(&components.ElasticsearchConnector{KafkaClient: kafkaClient})
	custodian.AddComponent(components.NewAvroSchema(components.AvroSchemaParameters{
		Registry: registryConfluentClient}))
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
	err = custodian.Scrub()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}
