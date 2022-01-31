package components

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	"strings"
)

type GraphQLSchema struct {
	schema     string
	id         string
	restClient *schemaregistry.RestClient
	name       string
	version    string
}

type GraphQLSchemaParameters struct {
	Schema   string
	Registry *schemaregistry.RestClient
}

func NewGraphQLSchema(parameters GraphQLSchemaParameters) *GraphQLSchema {
	return &GraphQLSchema{
		schema:     parameters.Schema,
		restClient: parameters.Registry,
	}
}

func (as *GraphQLSchema) SetName(name string) {
	as.name = strings.ToLower(name)
}

func (as *GraphQLSchema) SetVersion(version string) {
	as.version = version
}

func (as *GraphQLSchema) Name() string {
	return as.name + "." + as.version
}

func (as *GraphQLSchema) Create() (err error) {
	id, err := as.restClient.RegisterGraphQLSchema(as.Name())
	if err != nil {
		return errors.Wrap(err, 0)
	}

	as.id = id
	return
}

func (as *GraphQLSchema) Delete() (err error) {
	return as.restClient.DeleteGraphQLSchema(as.Name())
}

func (as *GraphQLSchema) DeleteByVersion(version string) (err error) {
	return as.restClient.DeleteGraphQLSchema(as.name + "." + version)
}

func (as *GraphQLSchema) CheckDeviation() (err error) {
	return //TODO
}

func (as *GraphQLSchema) Exists() (exists bool, err error) {
	exists, err = as.restClient.CheckIfGraphQLSchemaExists(as.Name())
	if err != nil {
		return false, errors.Wrap(err, 0)
	}
	return
}

func (as *GraphQLSchema) ListInstalledVersions() (installedVersions []string, err error) {
	installedVersions, err = as.restClient.ListVersionsForSchemaName(as.name)
	if err != nil {
		return installedVersions, errors.Wrap(err, 0)
	}
	return
}
