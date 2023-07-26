package components

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
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
	events     events.Events
	log        logger.Log
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

func (as *AvroSchema) SetLogger(log logger.Log) {
	as.log = log
}

func (as *AvroSchema) SetName(kind string, name string) {
	as.name = strings.ToLower(kind + "." + name)
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
		as.events.Warning("CreateAvroSchemaFailure",
			"Unable to set schema name namespace for avro schema %s", as.Name())
		return errors.Wrap(err, 0)
	}

	id, err := as.registry.RegisterAvroSchema(as.Name(), schema, as.references)
	if err != nil {
		as.events.Warning("CreateAvroSchemaFailure",
			"Unable to register avro schema %s", as.Name())
		return errors.Wrap(err, 0)
	}

	as.events.Normal("CreatedAvroSchema", "Avro schema %s was successfully created", as.Name())

	as.id = id
	return
}

func (as *AvroSchema) Delete() (err error) {
	err = as.registry.DeleteSchema(as.Name())
	if err != nil {
		as.events.Warning("DeleteAvroSchemaFailure",
			"Unable to delete avro schema %s", as.Name())
		return errors.Wrap(err, 0)
	}

	as.events.Normal("DeleteAvroSchema", "Avro schema %s was successfully deleted", as.Name())
	return
}

func (as *AvroSchema) DeleteByVersion(version string) (err error) {
	fullName := as.name + "." + version
	err = as.registry.DeleteSchema(fullName)
	if err != nil {
		as.events.Warning("DeleteAvroSchemaFailure",
			"Unable to delete avro schema %s by version %s", as.name, version)
		return errors.Wrap(err, 0)
	}
	as.events.Normal("DeleteAvroSchema", "Avro schema %s was successfully deleted", fullName)
	return
}

func (as *AvroSchema) CheckDeviation() (problem, err error) {
	expectedSchema, err := as.SetSchemaNameNamespace()
	if err != nil {
		as.events.Warning("AvroSchemaCheckDeviationFailed",
			"Unable to SetSchemaNameNamespace during CheckDeviation for AvroSchema: %s", as.Name())
		return nil, errors.Wrap(err, 0)
	}

	as.registry.Client.ResetCache()
	existingSchema, err := as.registry.Client.GetLatestSchema(as.Name())
	if err != nil {
		srErr, ok := err.(srclient.Error)
		if !ok {
			as.events.Warning("AvroSchemaCheckDeviationFailed",
				"Unexpected error type when getting latest schema for AvroSchema %s", as.Name())
			return nil, errors.Wrap(errors.New("invalid error type in AvroSchema.CheckDeviation"), 0)
		}
		if srErr.Code == 40401 {
			// Error code 40401 – Subject not found
			as.events.Warning("AvroSchemaDeviationFound",
				"AvroSchema %s not found in registry", as.Name())
			return fmt.Errorf("schema for subject %s not found in registry", as.Name()), nil
		}

		as.events.Warning("AvroSchemaCheckDeviationFailed",
			"Unable to get latest schema for AvroSchema %s", as.Name())
		return nil, errors.Wrap(err, 0)
	}

	diff := cmp.Diff(
		expectedSchema,
		existingSchema.Schema())

	if len(diff) > 0 {
		msg := fmt.Sprintf("schema in registry changed for subject %s, diff: %s", as.Name(), diff)
		as.events.Warning("AvroSchemaDeviationFound", msg)
		problem = fmt.Errorf(msg)
	}

	return
}

func (as *AvroSchema) Exists() (exists bool, err error) {
	exists, err = as.registry.CheckIfSchemaVersionExists(as.Name(), as.id)
	if err != nil {
		as.events.Warning("AvroSchemaExistsFailed",
			"Unable to check if schema version exists for AvroSchema %s", as.Name())
		return false, errors.Wrap(err, 0)
	}
	return
}

func (as *AvroSchema) ListInstalledVersions() (installedVersions []string, err error) {
	subjects, err := as.registry.Client.GetSubjects()
	if err != nil {
		as.events.Warning("AvroSchemaListInstalledVersionsFailed",
			"Unable to list installed versioned for AvroSchema %s", as.Name())
		return nil, errors.Wrap(err, 0)
	}

	for _, subject := range subjects {
		if strings.Index(subject, as.name+".") == 0 {
			version := strings.Split(subject, ".")[2]
			version = strings.Split(version, "-")[0]
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

func (as *AvroSchema) SetEvents(e events.Events) {
	as.events = e
}
