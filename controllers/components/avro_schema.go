package components

import "github.com/redhatinsights/xjoin-operator/controllers/avro"

type AvroSchema struct {
	schemaName string
	schema     string
	id         int
}

func NewAvroSchema(schemaName string, schema string) *AvroSchema {
	return &AvroSchema{
		schemaName: schemaName,
		schema:     schema,
	}
}

func (as *AvroSchema) Name() string {
	return "AvroSchema"
}

func (as *AvroSchema) Create() (err error) {
	registry := avro.NewSchemaRegistry(
		avro.SchemaRegistryConnectionParams{
			Protocol: "http",
			Hostname: "confluent-schema-registry",
			Port:     "8081",
		})

	registry.Init()

	id, err := registry.RegisterSchema(as.schemaName, as.schema)
	if err != nil {
		return
	}

	as.id = id
	return
}

func (as *AvroSchema) Delete() (err error) {
	return
}

func (as *AvroSchema) CheckDeviation() (err error) {
	return
}

func (as *AvroSchema) Exists() (exists bool, err error) {
	return
}
