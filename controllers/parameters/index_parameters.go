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

const ElasticsearchSecret = "elasticsearch"

type IndexParameters struct {
	CommonParameters
	PrometheusPushGatewayUrl         Parameter
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
	ValidationAttemptInterval        Parameter //period between failed validation attempts within the validation pod (seconds)
	ValidationAttempts               Parameter //number of validation attempts in the validation pod before validation fails
	SubgraphLogLevel                 Parameter //log level for the api-subgraph pods
	ValidationPodCPURequest          Parameter
	ValidationPodCPULimit            Parameter
	ValidationPodMemoryRequest       Parameter
	ValidationPodMemoryLimit         Parameter
	CorePodCPURequest                Parameter
	CorePodCPULimit                  Parameter
	CorePodMemoryRequest             Parameter
	CorePodMemoryLimit               Parameter
	CoreNumPods                      Parameter
	KafkaTopicPartitions             Parameter
	KafkaTopicReplicas               Parameter
	KafkaTopicCleanupPolicy          Parameter
	KafkaTopicMinCompactionLagMS     Parameter
	KafkaTopicRetentionBytes         Parameter
	KafkaTopicRetentionMS            Parameter
	KafkaTopicMessageBytes           Parameter
	KafkaTopicCreationTimeout        Parameter
}

func BuildIndexParameters() *IndexParameters {
	p := IndexParameters{
		PrometheusPushGatewayUrl: Parameter{
			DefaultValue:  "http://xjoin-prometheus-push-gateway:9091",
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "prometheus.push.gateway.url",
		},
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
			Secret:       ElasticsearchSecret,
			SecretKey:    []string{"endpoint"},
			DefaultValue: "http://localhost:9200",
			Ephemeral: func(manager Manager) (interface{}, error) {
				return "http://xjoin-elasticsearch-es-default." + manager.ResourceNamespace + ".svc:9200", nil
			},
		},
		ElasticSearchUsername: Parameter{
			Type:         reflect.String,
			Secret:       ElasticsearchSecret,
			SecretKey:    []string{"username"},
			DefaultValue: "xjoin",
			Ephemeral: func(manager Manager) (interface{}, error) {
				return "elastic", nil
			},
		},
		ElasticSearchPassword: Parameter{
			Type:         reflect.String,
			Secret:       ElasticsearchSecret,
			SecretKey:    []string{"password"},
			DefaultValue: "xjoin1337",
			Ephemeral: func(manager Manager) (interface{}, error) {
				ctx, cancel := xjoinUtils.DefaultContext()
				defer cancel()
				esSecret, err := k8sUtils.FetchSecret(
					manager.Client, manager.ResourceNamespace, "xjoin-elasticsearch-es-elastic-user", ctx)
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
					client.InNamespace(manager.ResourceNamespace))

				if err != nil {
					return nil, err
				}

				if len(kafka.Items) != 1 {
					return nil, errors.New("invalid number of kafka instances found: " + strconv.Itoa(len(kafka.Items)))
				}

				return kafka.Items[0].GetName() + "-kafka-bootstrap." + manager.ResourceNamespace + ".svc:9092", nil
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
		ValidationAttemptInterval: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "validation.attempt.interval",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  60,
		},
		ValidationAttempts: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "validation.attempts",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  5,
		},
		SubgraphLogLevel: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "subgraph.log.level",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "WARN",
		},
		ValidationPodCPURequest: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "validation.pod.cpu.request",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "100m",
		},
		ValidationPodCPULimit: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "validation.pod.cpu.limit",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "200m",
		},
		ValidationPodMemoryRequest: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "validation.pod.memory.request",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "128Mi",
		},
		ValidationPodMemoryLimit: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "validation.pod.memory.limit",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "256Mi",
		},

		CorePodCPURequest: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "core.pod.cpu.request",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "100m",
		},
		CorePodCPULimit: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "core.pod.cpu.limit",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "200m",
		},
		CorePodMemoryRequest: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "core.pod.memory.request",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "128Mi",
		},
		CorePodMemoryLimit: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "core.pod.memory.limit",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "256Mi",
		},
		CoreNumPods: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "core.number.of.pods",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},

		//kafka topic
		KafkaTopicPartitions: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "index.kafka.topic.partitions",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},
		KafkaTopicReplicas: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "index.kafka.topic.replicas",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},
		KafkaTopicCleanupPolicy: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "index.kafka.topic.cleanup.policy",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "compact,delete",
		},
		KafkaTopicMinCompactionLagMS: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "index.kafka.topic.min.compaction.lag.ms",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "3600000",
		},
		KafkaTopicRetentionBytes: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "index.kafka.topic.retention.bytes",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "5368709120",
		},
		KafkaTopicRetentionMS: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "index.kafka.topic.retention.ms",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "2678400001",
		},
		KafkaTopicMessageBytes: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "index.kafka.topic.max.message.bytes",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "2097176",
		},
		KafkaTopicCreationTimeout: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "index.kafka.topic.creation.timeout",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  300,
		},
	}

	p.CommonParameters = BuildCommonParameters()

	return &p
}
