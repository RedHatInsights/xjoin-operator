package components

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/avro"
	"github.com/riferrei/srclient"
	"strings"
)

type AvroSchema struct {
	schema     string
	id         int
	registry   *avro.SchemaRegistry
	name       string
	version    string
	references []srclient.Reference
}

func NewAvroSchema(schema string, references []srclient.Reference) *AvroSchema {
	registry := avro.NewSchemaRegistry(
		avro.SchemaRegistryConnectionParams{
			Protocol: "http",
			Hostname: "confluent-schema-registry.test.svc",
			Port:     "8081",
		})

	registry.Init()

	return &AvroSchema{
		schema:     schema,
		registry:   registry,
		references: references,
		id:         1, //valid avro ids start at 1
	}
}

func (as *AvroSchema) SetName(name string) {
	as.name = name
}

func (as *AvroSchema) SetVersion(version string) {
	as.version = version
}

func (as *AvroSchema) Name() string {
	return as.name + "." + as.version
}

func (as *AvroSchema) Create() (err error) {
	id, err := as.registry.RegisterSchema(as.Name(), as.schema, as.references)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	as.id = id
	return
}

func (as *AvroSchema) Delete() (err error) {
	return as.registry.DeleteSchema(as.Name())
}

func (as *AvroSchema) DeleteByVersion(version string) (err error) {
	return as.registry.DeleteSchema(as.name + "." + version)
}

func (as *AvroSchema) CheckDeviation() (err error) {
	return //TODO
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
