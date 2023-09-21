package parameters

import (
	"reflect"

	. "github.com/redhatinsights/xjoin-operator/controllers/config"
)

type DataSourceParameters struct {
	CommonParameters
	DatabaseHostname                     Parameter
	DatabasePort                         Parameter
	DatabaseName                         Parameter
	DatabaseTable                        Parameter
	DatabaseUsername                     Parameter
	DatabasePassword                     Parameter
	DatabaseSSLMode                      Parameter
	DatabaseSSLRootCert                  Parameter
	DebeziumConnectorTemplate            Parameter
	DebeziumTasksMax                     Parameter
	DebeziumMaxBatchSize                 Parameter
	DebeziumIncrementalSnapshotChunkSize Parameter
	DebeziumQueueSize                    Parameter
	DebeziumSnapshotFetchSize            Parameter
	DebeziumPollIntervalMS               Parameter
	DebeziumErrorsLogEnable              Parameter
	DebeziumConnectorLingerMS            Parameter
	DebeziumConnectorBatchSize           Parameter
	DebeziumConnectorCompressionType     Parameter
	DebeziumConnectorMaxRequestSize      Parameter
	DebeziumConnectorAcks                Parameter
	KafkaTopicPartitions                 Parameter
	KafkaTopicReplicas                   Parameter
	KafkaTopicCleanupPolicy              Parameter
	KafkaTopicMinCompactionLagMS         Parameter
	KafkaTopicRetentionBytes             Parameter
	KafkaTopicRetentionMS                Parameter
	KafkaTopicMessageBytes               Parameter
	KafkaTopicCreationTimeout            Parameter
}

func BuildDataSourceParameters() *DataSourceParameters {
	p := DataSourceParameters{

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
			DefaultValue: `{
				"linger.ms": "{{.DebeziumConnectorLingerMS}}",
				"batch.size": "{{.DebeziumConnectorBatchSize}}",
				"compression.type": "{{.DebeziumConnectorCompressionType}}",
				"max.request.size": "{{.DebeziumConnectorMaxRequestSize}}",
				"acks": "{{.DebeziumConnectorAcks}}",
				"tasks.max": "{{.DebeziumTasksMax}}",
				"database.hostname": "{{.DatabaseHostname}}",
				"database.port": "{{.DatabasePort}}",
				"database.user": "{{.DatabaseUsername}}",
				"database.password": "{{.DatabasePassword}}",
				"database.dbname": "{{.DatabaseName}}",
				"database.server.name": "{{.DatabaseServerName}}",
				"database.sslmode": "{{.DatabaseSSLMode}}",
				"database.sslrootcert": "{{.DatabaseSSLRootCert}}",
				"table.whitelist": "{{.DatabaseTable}}",
				"plugin.name": "pgoutput",
				"transforms": "unwrap, reroute",
				"transforms.unwrap.type": "io.debezium.transforms.ExtractNewRecordState",
				"transforms.unwrap.delete.handling.mode": "none",
				"transforms.unwrap.add.fields": "ts_ms,source.ts_ms",
				"transforms.unwrap.add.fields.prefix": "__dbz_",
				"errors.log.enable": {{.DebeziumErrorsLogEnable}},
				"errors.log.include.messages": true,
				"slot.name": "{{.ReplicationSlotName}}",
				"max.queue.size": {{.DebeziumQueueSize}},
				"max.batch.size": {{.DebeziumMaxBatchSize}},
				"incremental.snapshot.chunk.size": {{.DebeziumIncrementalSnapshotChunkSize}},
                "snapshot.fetch.size": {{.DebeziumSnapshotFetchSize}},
				"poll.interval.ms": {{.DebeziumPollIntervalMS}},
				"key.converter": "io.apicurio.registry.utils.converter.AvroConverter",
				"key.converter.apicurio.registry.url": "{{.SchemaRegistryProtocol}}://{{.SchemaRegistryHost}}:{{.SchemaRegistryPort}}/apis/registry/v2",
				"key.converter.apicurio.registry.auto-register": "true",
				"value.converter": "io.apicurio.registry.utils.converter.AvroConverter",
				"value.converter.apicurio.registry.url": "{{.SchemaRegistryProtocol}}://{{.SchemaRegistryHost}}:{{.SchemaRegistryPort}}/apis/registry/v2",
				"value.converter.apicurio.registry.auto-register": "false",
				"value.converter.apicurio.registry.find-latest": "true",
				"value.converter.enhanced.avro.schema.support": "true",
				"transforms.reroute.type": "io.debezium.transforms.ByLogicalTableRouter",
				"transforms.reroute.topic.regex": ".*",
				"transforms.reroute.topic.replacement": "{{.TopicName}}"
			}`,
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
		DebeziumIncrementalSnapshotChunkSize: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "debezium.connector.incremental.snapshot.chunk.size",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1024,
		},
		DebeziumQueueSize: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "debezium.connector.max.queue.size",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1000,
		},
		DebeziumSnapshotFetchSize: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "debezium.connector.snapshot.fetch.size",
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
		DebeziumConnectorLingerMS: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "debezium.connector.linger.ms",
			DefaultValue:  "100",
		},
		DebeziumConnectorBatchSize: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "debezium.connector.batch.size",
			DefaultValue:  "200000",
		},
		DebeziumConnectorCompressionType: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "debezium.connector.compression.type",
			DefaultValue:  "lz4",
		},
		DebeziumConnectorMaxRequestSize: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "debezium.connector.max.request.size",
			DefaultValue:  "104857600",
		},
		DebeziumConnectorAcks: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "debezium.connector.acks",
			DefaultValue:  "1",
		},

		//kafka topic
		KafkaTopicPartitions: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "datasource.kafka.topic.partitions",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},
		KafkaTopicReplicas: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "datasource.kafka.topic.replicas",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},
		KafkaTopicCleanupPolicy: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "datasource.kafka.topic.cleanup.policy",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "compact,delete",
		},
		KafkaTopicMinCompactionLagMS: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "datasource.kafka.topic.min.compaction.lag.ms",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "3600000",
		},
		KafkaTopicRetentionBytes: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "datasource.kafka.topic.retention.bytes",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "5368709120",
		},
		KafkaTopicRetentionMS: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "datasource.kafka.topic.retention.ms",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "2678400001",
		},
		KafkaTopicMessageBytes: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "datasource.kafka.topic.max.message.bytes",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "2097176",
		},
		KafkaTopicCreationTimeout: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "datasource.kafka.topic.creation.timeout",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  300,
		},
	}

	p.CommonParameters = BuildCommonParameters()

	return &p
}
