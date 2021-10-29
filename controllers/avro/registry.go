package avro

import (
	"encoding/json"
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
	Client *srclient.SchemaRegistryClient
	URL    string
}

func NewSchemaRegistry(connectionParams SchemaRegistryConnectionParams) *SchemaRegistry {
	return &SchemaRegistry{
		URL: fmt.Sprintf("%s://%s:%s", connectionParams.Protocol, connectionParams.Hostname, connectionParams.Port),
	}
}

func (sr *SchemaRegistry) Init() {
	sr.Client = srclient.CreateSchemaRegistryClient(sr.URL)
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

//ExpandReferences retrieves the full schema for each xjoinref field
func (sr *SchemaRegistry) ExpandReferences(baseSchema string, references []srclient.Reference) (schema map[string]interface{}, err error) {
	var baseSchemaMap map[string]interface{}
	err = json.Unmarshal([]byte(baseSchema), &baseSchemaMap)
	if err != nil {
		return schema, errors.Wrap(err, 0)
	}

	for _, f := range baseSchemaMap["fields"].([]interface{}) {
		field := f.(map[string]interface{})
		if field["xjoin.type"] == "reference" {
			ref, err := findReferenceByType(references, field["type"].(string))
			if err != nil {
				return schema, errors.Wrap(err, 0)
			}
			refSchema, err := sr.Client.GetLatestSchema(ref.Subject)
			if err != nil {
				return schema, errors.Wrap(err, 0)
			}

			var refSchemaMap map[string]interface{}
			err = json.Unmarshal([]byte(refSchema.Schema()), &refSchemaMap)
			if err != nil {
				return schema, errors.Wrap(err, 0)
			}

			refSchemaMap["xjoin.type"] = field["xjoin.type"]
			field["type"] = refSchemaMap
		}
	}

	return baseSchemaMap, nil
}

func findReferenceByType(references []srclient.Reference, refType string) (srclient.Reference, error) {
	for _, ref := range references {
		if ref.Name == refType {
			return ref, nil
		}
	}
	return srclient.Reference{}, errors.Wrap(errors.New("reference "+refType+"not found in list of references"), 0)
}
