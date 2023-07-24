package components

import (
	"fmt"
	"github.com/go-errors/errors"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
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
	events     events.Events
	log        logger.Log
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

func (as *GraphQLSchema) SetLogger(log logger.Log) {
	as.log = log
}

func (as *GraphQLSchema) NameSuffix() string {
	return as.suffix
}

func (as *GraphQLSchema) SetName(kind string, name string) {
	as.name = strings.ToLower(kind + "." + name)
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
		as.events.Warning("CreateGraphQLSchemaFailed",
			"Unable to create GraphQLSchema %s", as.Name())
		return errors.Wrap(err, 0)
	}

	as.events.Normal("CreatedGraphQLSchema",
		"GraphQLSchema %s was successfully created", as.Name())

	as.id = id
	return
}

func (as *GraphQLSchema) Delete() (err error) {
	err = as.restClient.DeleteGraphQLSchema(as.Name())
	if err != nil {
		as.events.Warning("DeleteGraphQLSchemaFailed",
			"Unable to delete GraphQLSchema %s", as.Name())
		return errors.Wrap(err, 0)
	}
	as.events.Normal("DeleteGraphQLSchema",
		"GraphQLSchema %s was successfully deleted", as.Name())
	return
}

func (as *GraphQLSchema) DeleteByVersion(version string) (err error) {
	fullName := as.name + "." + version
	err = as.restClient.DeleteGraphQLSchema(as.name + "." + version)
	if err != nil {
		as.events.Warning("DeleteGraphQLSchemaFailed",
			"Unable to delete GraphQLSchema %s", fullName)
		return errors.Wrap(err, 0)
	}
	as.events.Normal("DeleteGraphQLSchema",
		"GraphQLSchema %s was successfully deleted", fullName)
	return
}

func (as *GraphQLSchema) CheckDeviation() (problem, err error) {
	//validate schema exists, body is managed by subgraph
	found, err := as.Exists()
	if err != nil {
		as.events.Warning("GraphQLSchemaCheckDeviationFailed",
			"Unable to check if GraphQLSchema %s exists", as.Name())
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		as.events.Warning("GraphQLSchemaDeviationFound",
			"GraphQLSchema %s does not exist", as.Name())
		return fmt.Errorf("the GraphQL schema named, %s, does not exist", as.Name()), nil
	}

	//validate labels are correct
	existingLabels, err := as.restClient.GetSchemaLabels(as.Name())
	if err != nil {
		as.events.Warning("GraphQLSchemaCheckDeviationFailed",
			"Unable to get existing schema labels for GraphQLSchema %s", as.Name())
		return nil, errors.Wrap(err, 0)
	}

	//build expected labels
	expectedLabels := as.restClient.BuildGraphQLSchemaLabels(as.Name())

	//compare
	specDiff := cmp.Diff(
		existingLabels,
		expectedLabels)

	if len(specDiff) > 0 {
		as.events.Warning("GraphQLSchemaDeviationFound",
			"GraphQLSchema %s labels have changed", as.Name())
		return fmt.Errorf("graphql schema, %s, labels changed: %s", as.Name(), specDiff), nil
	}

	return
}

func (as *GraphQLSchema) Exists() (exists bool, err error) {
	exists, err = as.restClient.CheckIfGraphQLSchemaExists(as.Name())
	if err != nil {
		as.events.Warning("GraphQLSchemaExistsFailed",
			"Unable to CheckIfGraphQLSchemaExists for GraphQLSchema %s", as.Name())
		return false, errors.Wrap(err, 0)
	}
	return
}

func (as *GraphQLSchema) ListInstalledVersions() (installedVersions []string, err error) {
	installedVersions, err = as.restClient.ListVersionsForSchemaName(as.name)
	if err != nil {
		as.events.Warning("GraphQLSchemaListInstalledVersionsFailed",
			"Unable to ListVersionsForSchemaName for GraphQLSchema %s", as.Name())
		return installedVersions, errors.Wrap(err, 0)
	}
	return
}

func (as *GraphQLSchema) Reconcile() (err error) {
	if as.active {
		err = as.restClient.EnableSchema(as.Name())
		if err != nil {
			as.events.Warning("GraphQLSchemaReconcileFailed",
				"Unable to enable GraphQLSchema %s", as.Name())
			return errors.Wrap(err, 0)
		}

		as.events.Normal("GraphQLSchemaEnabled",
			"GraphQLSchema %s was enabled", as.Name())
	} else {
		err = as.restClient.DisableSchema(as.Name())
		if err != nil {
			as.events.Warning("GraphQLSchemaReconcileFailed",
				"Unable to disable GraphQLSchema %s", as.Name())
			return errors.Wrap(err, 0)
		}

		as.events.Normal("GraphQLSchemaDisabled",
			"GraphQLSchema %s was disabled", as.Name())
	}

	return nil
}

func (as *GraphQLSchema) SetEvents(e events.Events) {
	as.events = e
}
