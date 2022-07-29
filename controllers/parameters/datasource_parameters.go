package parameters

import (
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"reflect"
)

type DataSourceParameters struct {
	CommonParameters
	DatabaseHostname          Parameter
	DatabasePort              Parameter
	DatabaseName              Parameter
	DatabaseTable             Parameter
	DatabaseUsername          Parameter
	DatabasePassword          Parameter
	DatabaseSSLMode           Parameter
	DatabaseSSLRootCert       Parameter
	DebeziumConnectorTemplate Parameter
	DebeziumTasksMax          Parameter
	DebeziumMaxBatchSize      Parameter
	DebeziumQueueSize         Parameter
	DebeziumPollIntervalMS    Parameter
	DebeziumErrorsLogEnable   Parameter
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
				"transforms.unwrap.delete.handling.mode": "rewrite",
				"errors.log.enable": {{.DebeziumErrorsLogEnable}},
				"errors.log.include.messages": true,
				"slot.name": "{{.ReplicationSlotName}}",
				"max.queue.size": {{.DebeziumQueueSize}},
				"max.batch.size": {{.DebeziumMaxBatchSize}},
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
	}

	p.CommonParameters = BuildCommonParameters()

	return &p
}
