package components

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
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
		return errors.Wrap(err, 0)
	}
	return
}

func (es *ElasticsearchConnector) Delete() (err error) {
	err = es.KafkaClient.DeleteConnector(es.Name())
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (es *ElasticsearchConnector) CheckDeviation() (problem, err error) {
	//build the expected connector
	m := es.TemplateParameters
	m["Topic"] = es.Topic
	expectedConnector, err := es.KafkaClient.CreateGenericElasticsearchConnector(es.Name(), es.Namespace, es.Template, m, true)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	//get the already created (existing) connector
	found, err := es.Exists()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		return fmt.Errorf("the Elasticsearch connector named, %s, does not exist", es.Name()), nil
	}

	existingConnector, err := es.KafkaClient.GetConnector(es.Name())
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if existingConnector == nil {
		return fmt.Errorf("the Elasticsearch connector named, %s, was not found", es.Name()), nil
	} else {
		expectedTopicUnstructured := expectedConnector.UnstructuredContent()
		existingTopicUnstructured := existingConnector.UnstructuredContent()

		specDiff := cmp.Diff(
			expectedTopicUnstructured["spec"].(map[string]interface{}),
			existingTopicUnstructured["spec"].(map[string]interface{}),
			utils.NumberNormalizer)

		if len(specDiff) > 0 {
			return fmt.Errorf("elasticsearch connector spec has changed: %s", specDiff), nil
		}

		if existingConnector.GetNamespace() != expectedConnector.GetNamespace() {
			return fmt.Errorf(
				"elasticsearch connector namespace has changed from: %s to %s",
				existingConnector.GetNamespace(),
				expectedConnector.GetNamespace()), nil
		}
	}
	return
}

func (es *ElasticsearchConnector) Exists() (exists bool, err error) {
	exists, err = es.KafkaClient.CheckIfConnectorExists(es.Name())
	if err != nil {
		return false, errors.Wrap(err, 0)
	}
	return exists, nil
}

func (es *ElasticsearchConnector) ListInstalledVersions() (versions []string, err error) {
	installedConnectors, err := es.KafkaClient.ListConnectorNamesForPrefix(es.name)
	if err != nil {
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
