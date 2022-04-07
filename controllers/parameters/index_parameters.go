package parameters

import (
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"reflect"
)

type IndexParameters struct {
	CommonParameters
	ElasticSearchConnectorTemplate   Parameter
	ElasticSearchURL                 Parameter
	ElasticSearchUsername            Parameter
	ElasticSearchPassword            Parameter
	ElasticSearchTasksMax            Parameter
	ElasticSearchMaxInFlightRequests Parameter
	ElasticSearchErrorsLogEnable     Parameter
	ElasticSearchMaxRetries          Parameter
	ElasticSearchRetryBackoffMS      Parameter
	ElasticSearchBatchSize           Parameter
	ElasticSearchMaxBufferedRecords  Parameter
	ElasticSearchLingerMS            Parameter
	ElasticSearchNamespace           Parameter
	ElasticSearchSecretVersion       Parameter
	ElasticSearchPipelineTemplate    Parameter
	ElasticSearchIndexReplicas       Parameter
	ElasticSearchIndexShards         Parameter
	ElasticSearchIndexTemplate       Parameter
	KafkaBootstrapURL                Parameter
	CustomSubgraphImages             Parameter
}

func BuildIndexParameters() *IndexParameters {
	p := IndexParameters{
		ElasticSearchSecretVersion: Parameter{
			DefaultValue: "",
			Type:         reflect.String,
		},
		ElasticSearchIndexShards: Parameter{
			DefaultValue:  3,
			Type:          reflect.Int,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "elasticsearch.index.shards",
		},
		ElasticSearchIndexReplicas: Parameter{
			DefaultValue:  1,
			Type:          reflect.Int,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "elasticsearch.index.replicas",
		},
		ElasticSearchConnectorTemplate: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "elasticsearch.connector.template",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  ``,
		},
		ElasticSearchURL: Parameter{
			Type:         reflect.String,
			Secret:       "xjoin-elasticsearch",
			SecretKey:    []string{"endpoint"},
			DefaultValue: "http://localhost:9200",
		},
		ElasticSearchUsername: Parameter{
			Type:         reflect.String,
			Secret:       "xjoin-elasticsearch",
			SecretKey:    []string{"username"},
			DefaultValue: "xjoin",
		},
		ElasticSearchPassword: Parameter{
			Type:         reflect.String,
			Secret:       "xjoin-elasticsearch",
			SecretKey:    []string{"password"},
			DefaultValue: "xjoin1337",
		},
		ElasticSearchTasksMax: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "elasticsearch.connector.tasks.max",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},
		ElasticSearchMaxInFlightRequests: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "elasticsearch.connector.max.in.flight.requests",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},
		ElasticSearchErrorsLogEnable: Parameter{
			Type:          reflect.Bool,
			ConfigMapKey:  "elasticsearch.connector.errors.log.enable",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  true,
		},
		ElasticSearchMaxRetries: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "elasticsearch.connector.max.retries",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  8,
		},
		ElasticSearchRetryBackoffMS: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "elasticsearch.connector.retry.backoff.ms",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  100,
		},
		ElasticSearchBatchSize: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "elasticsearch.connector.batch.size",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  100,
		},
		ElasticSearchMaxBufferedRecords: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "elasticsearch.connector.max.buffered.records",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  500,
		},
		ElasticSearchLingerMS: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "elasticsearch.connector.linger.ms",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  100,
		},
		ElasticSearchIndexTemplate: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "elasticsearch.index.template",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "",
		},
		KafkaBootstrapURL: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.bootstrap.url",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "localhost:9092",
		},
		CustomSubgraphImages: Parameter{
			Type:         reflect.Slice,
			SpecKey:      "CustomSubgraphImages",
			DefaultValue: nil,
		},
	}

	p.CommonParameters = BuildCommonParameters()

	return &p
}
