package components

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-errors/errors"
	. "github.com/redhatinsights/xjoin-go-lib/pkg/avro"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	"github.com/riferrei/srclient"
)

type AvroSchema struct {
	schema     string
	id         int
	registry   *schemaregistry.ConfluentClient
	name       string
	version    string
	references []srclient.Reference
}

type AvroSchemaParameters struct {
	Schema     string
	Registry   *schemaregistry.ConfluentClient
	References []srclient.Reference
}

func NewAvroSchema(parameters AvroSchemaParameters) *AvroSchema {
	return &AvroSchema{
		schema:     parameters.Schema,
		registry:   parameters.Registry,
		references: parameters.References,
		id:         1, //valid avro ids start at 1
	}
}

func (as *AvroSchema) SetName(name string) {
	as.name = strings.ToLower(name)
}

func (as *AvroSchema) SetVersion(version string) {
	as.version = version
}

func (as *AvroSchema) Name() string {
	return as.name + "." + as.version + "-value"
}

func (as *AvroSchema) Create() (err error) {
	schema, err := as.SetSchemaNameNamespace()
	if err != nil {
		return errors.Wrap(err, 0)
	}

	id, err := as.registry.RegisterAvroSchema(as.Name(), schema, as.references)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	as.id = id
	return
}

func (as *AvroSchema) Delete() (err error) {
	err = as.registry.DeleteSchema(as.Name())
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (as *AvroSchema) DeleteByVersion(version string) (err error) {
	err = as.registry.DeleteSchema(as.name + "." + version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (as *AvroSchema) CheckDeviation() (problem, err error) {
	schema, err := as.SetSchemaNameNamespace()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	as.registry.Client.ResetCache()
	srschema, err := as.registry.Client.GetLatestSchema(as.Name())
	if err != nil {
		srerr, isSrerr := err.(srclient.Error)
		if isSrerr && srerr.Code == 40401 {
			// Error code 40401 – Subject not found
			return fmt.Errorf("schema for subject %s not found in registry", as.Name()), nil
		}
		return nil, errors.Wrap(err, 0)
	}

	if schema != srschema.Schema() {
		problem = fmt.Errorf("schema in registry changed for subject %s", as.Name())
	}

	return
}

func (as *AvroSchema) Exists() (exists bool, err error) {
	exists, err = as.registry.CheckIfSchemaVersionExists(as.Name(), as.id)
	if err != nil {
		return false, errors.Wrap(err, 0)
	}
	return
}

func (as *AvroSchema) ListInstalledVersions() (installedVersions []string, err error) {
	subjects, err := as.registry.Client.GetSubjects()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	for _, subject := range subjects {
		if strings.Index(subject, as.name+".") == 0 {
			version := strings.Split(subject, ".")[1]
			installedVersions = append(installedVersions, version)
		}
	}

	return
}

func (as *AvroSchema) SetSchemaNameNamespace() (schema string, err error) {
	var schemaObj Schema
	err = json.Unmarshal([]byte(as.schema), &schemaObj)
	if err != nil {
		return schema, errors.Wrap(err, 0)
	}

	schemaObj.Namespace = as.name
	schemaObj.Name = "Value"

	schemaBytes, err := json.Marshal(schemaObj)
	if err != nil {
		return schema, errors.Wrap(err, 0)
	}

	return string(schemaBytes), err
}

func (as *AvroSchema) Reconcile() (err error) {
	return nil
}
