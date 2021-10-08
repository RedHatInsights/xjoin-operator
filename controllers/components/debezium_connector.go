package components

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	"strings"
)

type DebeziumConnector struct {
	name               string
	version            string
	Template           string
	KafkaClient        kafka.GenericKafka
	TemplateParameters map[string]interface{}
}

func (dc *DebeziumConnector) SetName(name string) {
	dc.name = strings.ToLower(name)
}

func (dc *DebeziumConnector) SetVersion(version string) {
	dc.version = version
}

func (dc *DebeziumConnector) Name() string {
	return dc.name + "." + dc.version
}

func (dc *DebeziumConnector) Create() (err error) {
	m := dc.TemplateParameters
	m["DatabaseServerName"] = dc.Name()
	m["ReplicationSlotName"] = strings.ReplaceAll(dc.Name(), ".", "_")

	err = dc.KafkaClient.CreateGenericDebeziumConnector(dc.Name(), dc.Template, m)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (dc *DebeziumConnector) Delete() (err error) {
	err = dc.KafkaClient.DeleteConnector(dc.Name())
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (dc *DebeziumConnector) CheckDeviation() (err error) {
	return
}

func (dc *DebeziumConnector) Exists() (exists bool, err error) {
	exists, err = dc.KafkaClient.CheckIfConnectorExists(dc.Name())
	if err != nil {
		return false, errors.Wrap(err, 0)
	}
	return exists, nil
}

func (dc *DebeziumConnector) ListInstalledVersions() (versions []string, err error) {
	installedConnectors, err := dc.KafkaClient.ListConnectorNamesForPrefix(dc.name)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	for _, connector := range installedConnectors {
		versions = append(versions, strings.Split(connector, dc.name + ".")[1])
	}
	return
}
