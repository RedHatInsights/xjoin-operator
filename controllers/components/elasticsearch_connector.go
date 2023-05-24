package components

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	"strings"
)

type ElasticsearchConnector struct {
	name               string
	version            string
	Template           string
	KafkaClient        kafka.GenericKafka
	TemplateParameters map[string]interface{}
	Topic              string
}

func (es *ElasticsearchConnector) SetName(name string) {
	es.name = strings.ToLower(name)
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
