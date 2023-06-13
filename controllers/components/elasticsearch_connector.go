package components

import (
	"fmt"
	"strings"

	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ElasticsearchConnector struct {
	name               string
	version            string
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

	err = es.KafkaClient.CreateGenericElasticsearchConnector(es.Name(), es.Template, m)
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
	found, err := es.Exists()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		return fmt.Errorf("ElasticsearchConnector named, %s, does not exist.", es.Name()), nil
	}

	esConPtr, err := es.KafkaClient.GetConnector(es.Name())

	if err != nil {
		return nil, fmt.Errorf("Error encountered when getting ElasticsearchConnector: %w", err)
	}

	if esConPtr == nil {
		return fmt.Errorf("Problem encountered when getting ElasticsearchConnector: %w", err), nil
	} else {
		var allConns *unstructured.UnstructuredList
		allConns, err := es.KafkaClient.ListConnectors()

		if err != nil {
			return nil, fmt.Errorf("Error encountered when listing connectors: %w", err)
		}

		if allConns.Items == nil || len(allConns.Items) == 0 {
			return fmt.Errorf("No Elasticsearch connector available"), nil
		}

		for _, conn := range allConns.Items {
			if esConPtr.GetName() == conn.GetName() {
				if equality.Semantic.DeepEqual(*esConPtr, conn) {
					return nil, nil
				} else {
					return fmt.Errorf("Elasticsearch connector named %s has changed", conn.GetName()), nil
				}
			}
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
