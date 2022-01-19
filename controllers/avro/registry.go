package avro

import (
	"fmt"
	"github.com/go-errors/errors"
	"github.com/riferrei/srclient"
	"strings"
)

type SchemaRegistryConnectionParams struct {
	Protocol string
	Hostname string
	Port     string
}

type SchemaRegistry struct {
	Client          *srclient.SchemaRegistryClient
	confluentApiUrl string
	v1ApiUrl        string
	v2ApiUrl        string
}

func NewSchemaRegistry(connectionParams SchemaRegistryConnectionParams) *SchemaRegistry {
	baseUrl := fmt.Sprintf("%s://%s:%s", connectionParams.Protocol, connectionParams.Hostname, "1080")
	return &SchemaRegistry{
		confluentApiUrl: baseUrl + "/apis/ccompat/v6",
		v1ApiUrl:        baseUrl + "/apis/registry/v1",
		v2ApiUrl:        baseUrl + "/apis/registry/v2",
	}
}

func (sr *SchemaRegistry) Init() {
	sr.Client = srclient.CreateSchemaRegistryClient(sr.confluentApiUrl)
}

func (sr *SchemaRegistry) RegisterSchema(name string, schemaDefinition string, references []srclient.Reference) (id int, err error) {
	schema, err := sr.Client.CreateSchema(name, schemaDefinition, srclient.Avro, references...)
	if err != nil {
		return id, errors.Wrap(err, 0)
	}

	return schema.ID(), nil
}

func (sr *SchemaRegistry) CheckIfSchemaVersionExists(name string, version int) (exists bool, err error) {
	_, err = sr.Client.GetSchemaByVersion(name, version)
	if err != nil && strings.Index(err.Error(), "404 Not Found") == 0 {
		return false, nil
	} else if err != nil {
		return false, errors.Wrap(err, 0)
	} else {
		return true, nil
	}
}

func (sr *SchemaRegistry) DeleteSchema(name string) (err error) {
	return sr.Client.DeleteSubject(name, true)
}

func (sr *SchemaRegistry) GetSchemaReferences(subject string) (references []srclient.Reference, err error) {
	schema, err := sr.Client.GetLatestSchema(subject)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return schema.References(), nil
}

func (sr *SchemaRegistry) GetSchema(subject string) (schema string, err error) {
	schemaObj, err := sr.Client.GetLatestSchema(subject)
	if err != nil {
		return schema, errors.Wrap(err, 0)
	}
	return schemaObj.Schema(), nil
}
