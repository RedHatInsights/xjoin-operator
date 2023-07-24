package components

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"strings"

	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
)

type DebeziumConnector struct {
	name               string
	version            string
	Namespace          string
	Template           string
	KafkaClient        kafka.GenericKafka
	TemplateParameters map[string]interface{}
	events             events.Events
	log                logger.Log
}

func (dc *DebeziumConnector) SetLogger(log logger.Log) {
	dc.log = log
}

func (dc *DebeziumConnector) SetName(kind string, name string) {
	dc.name = strings.ToLower(kind + "." + name)
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
	m["TopicName"] = dc.Name()

	_, err = dc.KafkaClient.CreateGenericDebeziumConnector(dc.Name(), dc.Namespace, dc.Template, m, false)
	if err != nil {
		dc.events.Warning("CreateDebeziumConnectorFailure",
			"Unable to create DebeziumConnector %s", dc.Name())
		return errors.Wrap(err, 0)
	}

	dc.events.Normal("CreatedDebeziumConnector",
		"DebeziumConnector %s was successfully created", dc.Name())
	return
}

func (dc *DebeziumConnector) Delete() (err error) {
	err = dc.KafkaClient.DeleteConnector(dc.Name())
	if err != nil {
		dc.events.Warning("DeleteDebeziumConnectorFailure",
			"Unable to delete DebeziumConnector %s", dc.Name())
		return errors.Wrap(err, 0)
	}

	dc.events.Normal("DeletedDebeziumConnector",
		"DebeziumConnector %s was successfully deleted", dc.Name())
	return
}

func (dc *DebeziumConnector) CheckDeviation() (problem, err error) {
	//build the expected connector
	m := dc.TemplateParameters
	m["DatabaseServerName"] = dc.Name()
	m["ReplicationSlotName"] = strings.ReplaceAll(dc.Name(), ".", "_")
	m["TopicName"] = dc.Name()
	expectedConnector, err := dc.KafkaClient.CreateGenericDebeziumConnector(dc.Name(), dc.Namespace, dc.Template, m, true)
	if err != nil {
		dc.events.Warning("DebeziumConnectorCheckDeviationFailed",
			"Unable to create expected DebeziumConnector %s", dc.Name())
		return nil, errors.Wrap(err, 0)
	}

	//get the already created (existing) connector
	found, err := dc.Exists()
	if err != nil {
		dc.events.Warning("DebeziumConnectorCheckDeviationFailed",
			"Unable to check if DebeziumConnector %s exists", dc.Name())
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		dc.events.Warning("DebeziumConnectorDeviationFound",
			"DebeziumConnector %s does not exist", dc.Name())
		return fmt.Errorf("the Debezium connector named, %s, does not exist", dc.Name()), nil
	}

	existingConnector, err := dc.KafkaClient.GetConnector(dc.Name(), dc.Namespace)
	if err != nil {
		dc.events.Warning("DebeziumConnectorCheckDeviationFailed",
			"Unable to get DebeziumConnector %s", dc.Name())
		return nil, errors.Wrap(err, 0)
	}

	if existingConnector == nil {
		dc.events.Warning("DebeziumConnectorDeviationFound",
			"Existing DebeziumConnector %s was not found", dc.Name())
		return fmt.Errorf("the Debezium connector named, %s, was not found", dc.Name()), nil
	} else {
		expectedTopicUnstructured := expectedConnector.UnstructuredContent()
		existingTopicUnstructured := existingConnector.UnstructuredContent()

		specDiff := cmp.Diff(
			expectedTopicUnstructured["spec"].(map[string]interface{}),
			existingTopicUnstructured["spec"].(map[string]interface{}),
			utils.NumberNormalizer)

		if len(specDiff) > 0 {
			dc.events.Warning("DebeziumConnectorDeviationFound",
				"DebeziumConnector %s spec has changed", dc.Name())
			return fmt.Errorf("debezium connector spec has changed: %s", specDiff), nil
		}

		if existingConnector.GetNamespace() != expectedConnector.GetNamespace() {
			dc.events.Warning("DebeziumConnectorDeviationFound",
				"DebeziumConnector %s namespace has changed", dc.Name())
			return fmt.Errorf(
				"debezium connector namespace has changed from: %s to %s",
				existingConnector.GetNamespace(),
				expectedConnector.GetNamespace()), nil
		}
	}
	return
}

func (dc *DebeziumConnector) Exists() (exists bool, err error) {
	dc.log.Debug("Checking if DebeziumConnector exists", "name", dc.Name(), "namespace", dc.Namespace)
	exists, err = dc.KafkaClient.CheckIfConnectorExists(dc.Name(), dc.Namespace)
	if err != nil {
		dc.events.Warning("DebeziumConnectorExistsFailed",
			"Unable to check if DebeziumConnector %s exists", dc.Name())
		return false, errors.Wrap(err, 0)
	}
	return exists, nil
}

func (dc *DebeziumConnector) ListInstalledVersions() (versions []string, err error) {
	installedConnectors, err := dc.KafkaClient.ListConnectorNamesForPrefix(dc.name)
	if err != nil {
		dc.events.Warning("DebeziumConnectorListInstalledVersionsFailed",
			"Unable to ListConnectorNamesForPrefix for DebeziumConnector %s", dc.name)
		return nil, errors.Wrap(err, 0)
	}

	for _, connector := range installedConnectors {
		versions = append(versions, strings.Split(connector, dc.name+".")[1])
	}
	return
}

func (dc *DebeziumConnector) Reconcile() (err error) {
	return nil
}

func (dc *DebeziumConnector) SetEvents(e events.Events) {
	dc.events = e
}
