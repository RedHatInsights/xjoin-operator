package components

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	"strings"

	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
)

type ElasticsearchConnector struct {
	name               string
	version            string
	Namespace          string
	Template           string
	KafkaClient        kafka.GenericKafka
	TemplateParameters map[string]interface{}
	Topic              string
	events             events.Events
}

func (es *ElasticsearchConnector) SetName(kind string, name string) {
	es.name = strings.ToLower(kind + "." + name)
}

func (es *ElasticsearchConnector) SetVersion(version string) {
	es.version = version
}

func (es *ElasticsearchConnector) Name() string {
	return es.name + "." + es.version
}

func (es *ElasticsearchConnector) Create() (err error) {
	m := es.TemplateParameters
	m["Topic"] = es.Topic
	//m["RenameTopicReplacement"] = fmt.Sprintf("%s.%s", kafka.Parameters.ResourceNamePrefix.String(), pipelineVersion)

	_, err = es.KafkaClient.CreateGenericElasticsearchConnector(es.Name(), es.Namespace, es.Template, m, false)
	if err != nil {
		es.events.Warning("CreateElasticsearchConnectorFailure",
			"Unable to create ElasticsearchConnector %s", es.Name())
		return errors.Wrap(err, 0)
	}

	es.events.Normal("CreatedElasticsearchConnector",
		"ElasticsearchConnector %s was successfully created", es.Name())
	return
}

func (es *ElasticsearchConnector) Delete() (err error) {
	err = es.KafkaClient.DeleteConnector(es.Name())
	if err != nil {
		es.events.Warning("DeleteElasticsearchConnectorFailure",
			"Unable to delete ElasticsearchConnector %s", es.Name())
		return errors.Wrap(err, 0)
	}

	es.events.Normal("DeletedElasticsearchConnector",
		"ElasticsearchConnector %s was successfully deleted", es.Name())
	return
}

func (es *ElasticsearchConnector) CheckDeviation() (problem, err error) {
	//build the expected connector
	m := es.TemplateParameters
	m["Topic"] = es.Topic
	expectedConnector, err := es.KafkaClient.CreateGenericElasticsearchConnector(es.Name(), es.Namespace, es.Template, m, true)
	if err != nil {
		es.events.Warning("ElasticsearchConnectorCheckDeviationFailed",
			"Unable to create mock Elasticsearch connector %s", es.Name())
		return nil, errors.Wrap(err, 0)
	}

	//get the already created (existing) connector
	found, err := es.Exists()
	if err != nil {
		es.events.Warning("ElasticsearchConnectorCheckDeviationFailed",
			"Unable to check if ElasticsearchConnector %s exists", es.Name())
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		es.events.Warning("ElasticsearchConnectorDeviationFound",
			"ElasticsearchConnector %s does not exist", es.Name())
		return fmt.Errorf("the Elasticsearch connector named, %s, does not exist", es.Name()), nil
	}

	existingConnector, err := es.KafkaClient.GetConnector(es.Name(), es.Namespace)
	if err != nil {
		es.events.Warning("ElasticsearchConnectorCheckDeviationFailed",
			"Unable to get ElasticsearchConnector %s", es.Name())
		return nil, errors.Wrap(err, 0)
	}

	if existingConnector == nil {
		es.events.Warning("ElasticsearchConnectorDeviationFound",
			"Existing ElasticsearchConnector %s was not found", es.Name())
		return fmt.Errorf("the Elasticsearch connector named, %s, was not found", es.Name()), nil
	} else {
		expectedTopicUnstructured := expectedConnector.UnstructuredContent()
		existingTopicUnstructured := existingConnector.UnstructuredContent()

		specDiff := cmp.Diff(
			expectedTopicUnstructured["spec"].(map[string]interface{}),
			existingTopicUnstructured["spec"].(map[string]interface{}),
			utils.NumberNormalizer)

		if len(specDiff) > 0 {
			es.events.Warning("ElasticsearchConnectorDeviationFound",
				"ElasticsearchConnector %s spec has changed", es.Name())
			return fmt.Errorf("elasticsearch connector spec has changed: %s", specDiff), nil
		}

		if existingConnector.GetNamespace() != expectedConnector.GetNamespace() {
			es.events.Warning("ElasticsearchConnectorDeviationFound",
				"ElasticsearchConnector %s namespace has changed", es.Name())
			return fmt.Errorf(
				"elasticsearch connector namespace has changed from: %s to %s",
				existingConnector.GetNamespace(),
				expectedConnector.GetNamespace()), nil
		}
	}
	return
}

func (es *ElasticsearchConnector) Exists() (exists bool, err error) {
	exists, err = es.KafkaClient.CheckIfConnectorExists(es.Name(), es.Namespace)
	if err != nil {
		es.events.Warning("ElasticsearchConnectorExistsFailed",
			"Unable to check if ElasticsearchConnector %s exists", es.Name())
		return false, errors.Wrap(err, 0)
	}
	return exists, nil
}

func (es *ElasticsearchConnector) ListInstalledVersions() (versions []string, err error) {
	installedConnectors, err := es.KafkaClient.ListConnectorNamesForPrefix(es.name)
	if err != nil {
		es.events.Warning("ElasticsearchConnectorListInstalledVersionsFailed",
			"Unable to ListConnectorNamesForPrefix for ElasticsearchConnector %s", es.Name())
		return nil, errors.Wrap(err, 0)
	}

	for _, connector := range installedConnectors {
		versions = append(versions, strings.Split(connector, es.name+".")[1])
	}
	return
}

func (es *ElasticsearchConnector) Reconcile() (err error) {
	return nil
}

func (es *ElasticsearchConnector) SetEvents(e events.Events) {
	es.events = e
}
