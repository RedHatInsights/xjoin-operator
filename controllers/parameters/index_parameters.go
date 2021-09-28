package parameters

import (
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"reflect"
)

type IndexParameters struct {
	Pause                  Parameter
	SchemaRegistryProtocol Parameter
	SchemaRegistryHost     Parameter
	SchemaRegistryPort     Parameter
	AvroSchema             Parameter
	Version                Parameter
}

func BuildIndexParameters() *IndexParameters {
	p := IndexParameters{
		Pause: Parameter{
			SpecKey:      "Pause",
			DefaultValue: false,
			Type:         reflect.Bool,
		},
		SchemaRegistryProtocol: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin",
			ConfigMapKey:  "schemaregistry.protocol",
			DefaultValue:  "http",
		},
		SchemaRegistryHost: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin",
			ConfigMapKey:  "schemaregistry.host",
			DefaultValue:  "confluent-schema-registry.test.svc",
		},
		SchemaRegistryPort: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin",
			ConfigMapKey:  "schemaregistry.port",
			DefaultValue:  "8081",
		},
		AvroSchema: Parameter{
			Type:         reflect.String,
			SpecKey:      "AvroSchema",
			DefaultValue: "{}",
		},
		Version: Parameter{
			Type:         reflect.String,
			SpecKey:      "Version",
			DefaultValue: "1",
		},
	}

	return &p
}
