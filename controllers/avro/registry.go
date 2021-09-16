package avro

import (
	"fmt"
	"github.com/riferrei/srclient"
)

type SchemaRegistryConnectionParams struct {
	Protocol string
	Hostname string
	Port     string
}

type SchemaRegistry struct {
	Client *srclient.SchemaRegistryClient
	URL    string
}

func NewSchemaRegistry(connectionParams SchemaRegistryConnectionParams) *SchemaRegistry {
	return &SchemaRegistry{
		URL: fmt.Sprintf("%s://%s:%s", connectionParams.Protocol, connectionParams.Hostname, connectionParams.Port),
	}
}

func (sr *SchemaRegistry) Init() {
	sr.Client = srclient.CreateSchemaRegistryClient("http://localhost:8081")
}

func (sr *SchemaRegistry) RegisterSchema(name string, schemaDefinition string) (id int, err error) {
	schema, err := sr.Client.CreateSchema(name, schemaDefinition, srclient.Avro)
	if err != nil {
		return
	}
	return schema.ID(), nil
}
