package parameters

import (
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"reflect"
)

type CommonParameters struct {
	Pause                        Parameter
	Version                      Parameter
	ConnectCluster               Parameter
	ConnectClusterNamespace      Parameter
	KafkaTopicPartitions         Parameter
	KafkaTopicReplicas           Parameter
	KafkaTopicCleanupPolicy      Parameter
	KafkaTopicMinCompactionLagMS Parameter
	KafkaTopicRetentionBytes     Parameter
	KafkaTopicRetentionMS        Parameter
	KafkaTopicMessageBytes       Parameter
	KafkaTopicCreationTimeout    Parameter
	KafkaCluster                 Parameter
	KafkaClusterNamespace        Parameter
	SchemaRegistryProtocol       Parameter
	SchemaRegistryHost           Parameter
	SchemaRegistryPort           Parameter
	AvroSchema                   Parameter
}

func BuildCommonParameters() CommonParameters {
	p := CommonParameters{
		Pause: Parameter{
			SpecKey:      "Pause",
			DefaultValue: false,
			Type:         reflect.Bool,
		},
		Version: Parameter{
			Type:         reflect.String,
			SpecKey:      "Version",
			DefaultValue: "1",
		},

		//connect cluster
		ConnectCluster: Parameter{
			SpecKey:       "ConnectCluster",
			ConfigMapKey:  "connect.cluster",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "connect",
			Type:          reflect.String,
		},
		ConnectClusterNamespace: Parameter{
			SpecKey:       "ConnectClusterNamespace",
			ConfigMapKey:  "connect.cluster.namespace",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "test",
			Type:          reflect.String,
		},

		//kafka cluster
		KafkaCluster: Parameter{
			SpecKey:       "KafkaCluster",
			ConfigMapKey:  "kafka.cluster",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "kafka",
			Type:          reflect.String,
		},
		KafkaClusterNamespace: Parameter{
			SpecKey:       "KafkaClusterNamespace",
			ConfigMapKey:  "kafka.cluster.namespace",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "test",
			Type:          reflect.String,
		},

		//kafka topic
		KafkaTopicPartitions: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "kafka.topic.partitions",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},
		KafkaTopicReplicas: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "kafka.topic.replicas",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},
		KafkaTopicCleanupPolicy: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.topic.cleanup.policy",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "compact,delete",
		},
		KafkaTopicMinCompactionLagMS: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.topic.min.compaction.lag.ms",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "3600000",
		},
		KafkaTopicRetentionBytes: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.topic.retention.bytes",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "5368709120",
		},
		KafkaTopicRetentionMS: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.topic.retention.ms",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "2678400001",
		},
		KafkaTopicMessageBytes: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.topic.max.message.bytes",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "2097176",
		},
		KafkaTopicCreationTimeout: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "kafka.topic.creation.timeout",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  300,
		},

		//avro schema
		SchemaRegistryProtocol: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "schemaregistry.protocol",
			DefaultValue:  "http",
		},
		SchemaRegistryHost: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "schemaregistry.host",
			DefaultValue:  "apicurio.test.svc",
		},
		SchemaRegistryPort: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "schemaregistry.port",
			DefaultValue:  "10001",
		},
		AvroSchema: Parameter{
			Type:         reflect.String,
			SpecKey:      "AvroSchema",
			DefaultValue: "{}",
		},
	}

	return p
}
