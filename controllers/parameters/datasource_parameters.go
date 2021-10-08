package parameters

import (
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"reflect"
)

type DataSourceParameters struct {
	Pause                        Parameter
	SchemaRegistryProtocol       Parameter
	SchemaRegistryHost           Parameter
	SchemaRegistryPort           Parameter
	DatabaseHostname             Parameter
	DatabasePort                 Parameter
	DatabaseName                 Parameter
	DatabaseTable                Parameter
	DatabaseUsername             Parameter
	DatabasePassword             Parameter
	DatabaseSSLMode              Parameter
	DatabaseSSLRootCert          Parameter
	AvroSchema                   Parameter
	Version                      Parameter
	DebeziumConnectorTemplate    Parameter
	DebeziumTasksMax             Parameter
	DebeziumMaxBatchSize         Parameter
	DebeziumQueueSize            Parameter
	DebeziumPollIntervalMS       Parameter
	DebeziumErrorsLogEnable      Parameter
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
}

func BuildDataSourceParameters() *DataSourceParameters {
	p := DataSourceParameters{
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
			DefaultValue:  "confluent-schema-registry.test.svc",
		},
		SchemaRegistryPort: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "schemaregistry.port",
			DefaultValue:  "8081",
		},
		AvroSchema: Parameter{
			Type:         reflect.String,
			SpecKey:      "AvroSchema",
			DefaultValue: "{}",
		},

		//database
		DatabaseHostname: Parameter{
			Type:         reflect.String,
			SpecKey:      "DatabaseHostname",
			DefaultValue: "localhost",
		},
		DatabasePort: Parameter{
			Type:         reflect.String,
			SpecKey:      "DatabasePort",
			DefaultValue: "5432",
		},
		DatabaseName: Parameter{
			Type:         reflect.String,
			SpecKey:      "DatabaseName",
			DefaultValue: "insights",
		},
		DatabaseTable: Parameter{
			Type:         reflect.String,
			SpecKey:      "DatabaseTable",
			DefaultValue: "public.hosts",
		},
		DatabaseUsername: Parameter{
			Type:         reflect.String,
			SpecKey:      "DatabaseUsername",
			DefaultValue: "insights",
		},
		DatabasePassword: Parameter{
			Type:         reflect.String,
			SpecKey:      "DatabasePassword",
			DefaultValue: "insights",
		},
		DatabaseSSLMode: Parameter{
			Type:          reflect.String,
			DefaultValue:  "disable",
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "db.ssl.mode",
		},
		DatabaseSSLRootCert: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "hbi.db.ssl.root.cert",
			DefaultValue:  "/opt/kafka/external-configuration/rds-client-ca/rds-cacert",
		},

		//debezium
		DebeziumConnectorTemplate: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "debezium.connector.template",
			DefaultValue:  "",
		},
		DebeziumTasksMax: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "debezium.connector.tasks.max",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},
		DebeziumMaxBatchSize: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "debezium.connector.max.batch.size",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  10,
		},
		DebeziumQueueSize: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "debezium.connector.max.queue.size",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1000,
		},
		DebeziumPollIntervalMS: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "debezium.connector.poll.interval.ms",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  100,
		},
		DebeziumErrorsLogEnable: Parameter{
			Type:          reflect.Bool,
			ConfigMapKey:  "debezium.connector.errors.log.enable",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  true,
		},

		//kafka connect
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

		//kafka topic
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
	}

	return &p
}
