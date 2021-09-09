package components

import "github.com/redhatinsights/xjoin-operator/controllers/avro"

type AvroSchema struct {
	name   string
	schema string
	id     int
}

func NewAvroSchema(name string, schema string) *AvroSchema {
	return &AvroSchema{
		name:   name,
		schema: schema,
	}
}

func (as *AvroSchema) Name() string {
	return as.name
}

func (as *AvroSchema) Create() (err error) {
	registry := avro.NewSchemaRegistry(
		avro.SchemaRegistryConnectionParams{
			Protocol: "http",
			Hostname: "confluent-schema-registry",
			Port:     "8081",
		})

	registry.Init()

	id, err := registry.RegisterSchema(as.name, as.schema)
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
