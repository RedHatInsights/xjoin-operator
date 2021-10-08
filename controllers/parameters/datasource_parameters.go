package parameters

import (
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"reflect"
)

type DataSourceParameters struct {
	Pause                     Parameter
	SchemaRegistryProtocol    Parameter
	SchemaRegistryHost        Parameter
	SchemaRegistryPort        Parameter
	DatabaseHostname          Parameter
	DatabasePort              Parameter
	DatabaseName              Parameter
	DatabaseTable             Parameter
	DatabaseUsername          Parameter
	DatabasePassword          Parameter
	DatabaseSSLMode           Parameter
	DatabaseSSLRootCert       Parameter
	AvroSchema                Parameter
	Version                   Parameter
	DebeziumConnectorTemplate Parameter
	DebeziumTasksMax          Parameter
	DebeziumMaxBatchSize      Parameter
	DebeziumQueueSize         Parameter
	DebeziumPollIntervalMS    Parameter
	DebeziumErrorsLogEnable   Parameter
	ConnectCluster            Parameter
	ConnectClusterNamespace   Parameter
}

func BuildDataSourceParameters() *DataSourceParameters {
	p := DataSourceParameters{
		Pause: Parameter{
			SpecKey:      "Pause",
			DefaultValue: false,
			Type:         reflect.Bool,
		},
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
			DefaultValue: "hosts",
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
		Version: Parameter{
			Type:         reflect.String,
			SpecKey:      "Version",
			DefaultValue: "1",
		},
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
	}

	return &p
}
