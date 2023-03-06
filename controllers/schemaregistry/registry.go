package schemaregistry

import (
	"fmt"
	"reflect"

	"github.com/go-errors/errors"
	"github.com/riferrei/srclient"
)

type ConfluentClient struct {
	Client          *srclient.SchemaRegistryClient
	confluentApiUrl string
	v2ApiUrl        string
	artifactsUrl    string
}

func NewSchemaRegistryConfluentClient(connectionParams ConnectionParams) *ConfluentClient {
	baseUrl := fmt.Sprintf("%s://%s:%s", connectionParams.Protocol, connectionParams.Hostname, connectionParams.Port)
	return &ConfluentClient{
		confluentApiUrl: baseUrl + "/apis/ccompat/v6",
		v2ApiUrl:        baseUrl + "/apis/registry/v2",
		artifactsUrl:    baseUrl + "/api/artifacts",
	}
}

func (sr *ConfluentClient) Init() {
	sr.Client = srclient.CreateSchemaRegistryClient(sr.confluentApiUrl)
}

func (sr *ConfluentClient) RegisterAvroSchema(name string, schemaDefinition string, references []srclient.Reference) (id int, err error) {
	schema, err := sr.Client.CreateSchema(name, schemaDefinition, srclient.Avro, references...)
	if err != nil {
		return id, errors.Wrap(err, 0)
	}

	return schema.ID(), nil
}

func (sr *ConfluentClient) CheckIfSchemaVersionExists(name string, version int) (exists bool, err error) {
	_, err = sr.Client.GetSchemaByVersion(name, version)
	errorCode := -1
	if reflect.TypeOf(err) == reflect.TypeOf(srclient.Error{}) {
		errorCode = err.(srclient.Error).Code
	}

	if err != nil && (errorCode == 40402) {
		return false, nil
	} else if err != nil {
		return false, errors.Wrap(err, 0)
	} else {
		return true, nil
	}
}

func (sr *ConfluentClient) DeleteSchema(name string) (err error) {
	_, err = sr.Client.GetLatestSchema(name)

	errorCode := -1
	if reflect.TypeOf(err) == reflect.TypeOf(srclient.Error{}) {
		errorCode = err.(srclient.Error).Code
	}

	if err != nil && errorCode == 40403 {
		return nil //schema doesn't exist, don't try to delete it
	} else if err != nil {
		return errors.Wrap(err, 0)
	} else {
		err = sr.Client.DeleteSubject(name, false)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}
	return
}

func (sr *ConfluentClient) GetSchemaReferences(subject string) (references []srclient.Reference, err error) {
	schema, err := sr.Client.GetLatestSchema(subject)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return schema.References(), nil
}

func (sr *ConfluentClient) GetSchema(subject string) (schema string, err error) {
	schemaObj, err := sr.Client.GetLatestSchema(subject)
	if err != nil {
		return schema, errors.Wrap(err, 0)
	}
	return schemaObj.Schema(), nil
}
