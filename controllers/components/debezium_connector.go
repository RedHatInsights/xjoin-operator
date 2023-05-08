package components

import (
	"fmt"
	"strings"

	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	m["TopicName"] = dc.Name()

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

func (dc *DebeziumConnector) CheckDeviation() (problem, err error) {
	found, err := dc.Exists()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		return fmt.Errorf("DebeziumConnector named, %s, does not exist.", dc.Name()), nil
	}

	// leave this commented line for testing
	// debConPtr, err := dc.KafkaClient.GetConnector("xjoinindexpipeline.hosts.1679941693938094928")
	debConPtr, err := dc.KafkaClient.GetConnector(dc.Name())

	if err != nil {
		return nil, fmt.Errorf("Error encountered when getting DebeziumConnector: %w", err)
	}

	if debConPtr == nil {
		return fmt.Errorf("Error encountered when getting DebeziumConnector: %w", err), nil
	} else {
		var allConns *unstructured.UnstructuredList
		allConns, err := dc.KafkaClient.ListConnectors()
		if err != nil {
			return nil, fmt.Errorf("Error encountered when listing connectors: %w", err)
		}

		if allConns.Items == nil || len(allConns.Items) == 0 {
			return fmt.Errorf("No Debezium connector available"), nil
		}

		for _, conn := range allConns.Items {
			if debConPtr.GetName() == conn.GetName() {
				if equality.Semantic.DeepEqual(*debConPtr, conn) {
					return nil, nil
				} else {
					return fmt.Errorf("Debezium connector named %s has changed", conn.GetName()), nil
				}
			}
		}
	}
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
		versions = append(versions, strings.Split(connector, dc.name+".")[1])
	}
	return
}
