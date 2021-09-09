package parameters

import (
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"reflect"
)

const (
	PAUSE                    = "Pause"
	SCHEMA_REGISTRY_PROTOCOL = "SchemaRegistryProtocol"
	SCHEMA_REGISTRY_HOST     = "SchemaRegistryHost"
	SCHEMA_REGISTRY_PORT     = "SchemaRegistryPort"
	AVRO_SCHEMA              = "AvroSchema"
)

func BuildDataSourceParameters() map[string]Parameter {
	parameters := make(map[string]Parameter)

	parameters[PAUSE] = Parameter{
		SpecKey:      "Pause",
		DefaultValue: false,
		Type:         reflect.Bool,
	}
	parameters[SCHEMA_REGISTRY_PROTOCOL] = Parameter{
		Type:          reflect.String,
		ConfigMapName: "xjoin",
		ConfigMapKey:  "schemaregistry.protocol",
		DefaultValue:  "http",
	}
	parameters[SCHEMA_REGISTRY_HOST] = Parameter{
		Type:          reflect.String,
		ConfigMapName: "xjoin",
		ConfigMapKey:  "schemaregistry.host",
		DefaultValue:  "confluent-schema-registry.test.svc",
	}
	parameters[SCHEMA_REGISTRY_PORT] = Parameter{
		Type:          reflect.String,
		ConfigMapName: "xjoin",
		ConfigMapKey:  "schemaregistry.port",
		DefaultValue:  "8081",
	}
	parameters[AVRO_SCHEMA] = Parameter{
		Type:         reflect.String,
		SpecKey:      "AvroSchema",
		DefaultValue: "{}",
	}

	return parameters
}
