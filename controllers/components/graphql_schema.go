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
	suffix     string
	active     bool
}

type GraphQLSchemaParameters struct {
	Schema   string
	Registry *schemaregistry.RestClient
	Suffix   string
	Active   bool
}

func NewGraphQLSchema(parameters GraphQLSchemaParameters) *GraphQLSchema {
	return &GraphQLSchema{
		schema:     parameters.Schema,
		restClient: parameters.Registry,
		suffix:     parameters.Suffix,
		active:     parameters.Active,
	}
}

func (as *GraphQLSchema) NameSuffix() string {
	return as.suffix
}

func (as *GraphQLSchema) SetName(name string) {
	as.name = strings.ToLower(name)
	if as.suffix != "" {
		as.name = as.name + "-" + as.suffix
	}
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

func (as *GraphQLSchema) CheckDeviation() (problem, err error) {
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

func (as *GraphQLSchema) Reconcile() (err error) {
	if as.active {
		err = as.restClient.EnableSchema(as.Name())
		if err != nil {
			return errors.Wrap(err, 0)
		}
	} else {
		err = as.restClient.DisableSchema(as.Name())
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return nil
}
