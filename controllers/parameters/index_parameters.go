package parameters

import (
	"github.com/go-errors/errors"
	xjoinUtils "github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
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
	ValidationInterval               Parameter //period between validation checks (seconds)
	ValidationPodStatusInterval      Parameter //period between checking the status of the validation pod (seconds)
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
			DefaultValue: `{
			  "tasks.max": "{{.ElasticSearchTasksMax}}",
			  "topics": "{{.Topic}}",
			  "key.ignore": "false",
			  "connection.url": "{{.ElasticSearchURL}}",
			  {{if .ElasticSearchUsername}}"connection.username": "{{.ElasticSearchUsername}}",{{end}}
			  {{if .ElasticSearchPassword}}"connection.password": "{{.ElasticSearchPassword}}",{{end}}
			  "type.name": "_doc",
			  "transforms.deleteIf.type": "com.redhat.insights.deleteifsmt.DeleteIf$Value",
			  "transforms.deleteIf.field": "__deleted",
			  "transforms.deleteIf.value": "true",
			  "behavior.on.null.values":"delete",
			  "behavior.on.malformed.documents": "warn",
			  "auto.create.indices.at.start": false,
			  "schema.ignore": true,
			  "max.in.flight.requests": {{.ElasticSearchMaxInFlightRequests}},
			  "errors.log.enable": {{.ElasticSearchErrorsLogEnable}},
			  "errors.log.include.messages": true,
			  "max.retries": {{.ElasticSearchMaxRetries}},
			  "retry.backoff.ms": {{.ElasticSearchRetryBackoffMS}},
			  "batch.size": {{.ElasticSearchBatchSize}},
			  "max.buffered.records": {{.ElasticSearchMaxBufferedRecords}},
			  "linger.ms": {{.ElasticSearchLingerMS}},
			  "key.converter": "org.apache.kafka.connect.storage.StringConverter",
			  "value.converter": "io.apicurio.registry.utils.converter.AvroConverter",
			  "value.converter.apicurio.registry.auto-register": "false",
			  "value.converter.apicurio.registry.find-latest": "true",
			  "value.converter.apicurio.registry.url": "{{.SchemaRegistryProtocol}}://{{.SchemaRegistryHost}}:{{.SchemaRegistryPort}}/apis/registry/v2",
			  "value.converter.enhanced.avro.schema.support": "true"
			}`,
		},
		ElasticSearchURL: Parameter{
			Type:         reflect.String,
			Secret:       "xjoin-elasticsearch",
			SecretKey:    []string{"endpoint"},
			DefaultValue: "http://localhost:9200",
			Ephemeral: func(manager Manager) (interface{}, error) {
				return "http://xjoin-elasticsearch-es-default." + manager.Namespace + ".svc:9200", nil
			},
		},
		ElasticSearchUsername: Parameter{
			Type:         reflect.String,
			Secret:       "xjoin-elasticsearch",
			SecretKey:    []string{"username"},
			DefaultValue: "xjoin",
			Ephemeral: func(manager Manager) (interface{}, error) {
				return "elastic", nil
			},
		},
		ElasticSearchPassword: Parameter{
			Type:         reflect.String,
			Secret:       "xjoin-elasticsearch",
			SecretKey:    []string{"password"},
			DefaultValue: "xjoin1337",
			Ephemeral: func(manager Manager) (interface{}, error) {
				ctx, cancel := xjoinUtils.DefaultContext()
				defer cancel()
				esSecret, err := k8sUtils.FetchSecret(
					manager.Client, manager.Namespace, "xjoin-elasticsearch-es-elastic-user", ctx)
				if err != nil {
					return nil, err
				}

				password, err := k8sUtils.ReadSecretValue(esSecret, []string{"elastic"})
				if err != nil {
					return nil, err
				}
				return password, nil
			},
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
			DefaultValue: `
				{
				  "settings": {
					  "index": {
						  "number_of_shards": "{{.ElasticSearchIndexShards}}",
						  "number_of_replicas": "{{.ElasticSearchIndexReplicas}}",
						  "max_result_window": 100000,
						  "default_pipeline": "{{.ElasticSearchPipeline}}",
						  "analysis": {
							  "normalizer": {
								  "case_insensitive": {
									  "filter": "lowercase"
								  }
							  }
						  }
					  }
				  },
				  "mappings": {
					  "dynamic": "false",
					  "properties": {{.ElasticSearchProperties}}
				  }
				}
			`,
		},
		KafkaBootstrapURL: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.bootstrap.url",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "localhost:9092",
			Ephemeral: func(manager Manager) (interface{}, error) {
				var kafkaGVK = schema.GroupVersionKind{
					Group:   "kafka.strimzi.io",
					Kind:    "KafkaList",
					Version: "v1beta2",
				}

				kafka := &unstructured.UnstructuredList{}
				kafka.SetGroupVersionKind(kafkaGVK)

				ctx, cancel := xjoinUtils.DefaultContext()
				defer cancel()
				err := manager.Client.List(
					ctx,
					kafka,
					client.InNamespace(manager.Namespace))

				if err != nil {
					return nil, err
				}

				if len(kafka.Items) != 1 {
					return nil, errors.New("invalid number of kafka instances found: " + strconv.Itoa(len(kafka.Items)))
				}

				return kafka.Items[0].GetName() + "-kafka-bootstrap." + manager.Namespace + ".svc:9092", nil
			},
		},
		CustomSubgraphImages: Parameter{
			Type:         reflect.Slice,
			SpecKey:      "CustomSubgraphImages",
			DefaultValue: nil,
		},
		ValidationInterval: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "validation.interval",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1 * 60,
		},
		ValidationPodStatusInterval: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "validation.pod.status.interval",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  5,
		},
	}

	p.CommonParameters = BuildCommonParameters()

	return &p
}
