package components

import "github.com/redhatinsights/xjoin-operator/controllers/kafka"

type DebeziumConnector struct {
	name        string
	version     string
	template    string
	kafkaClient kafka.Kafka
}

func NewDebeziumConnector() *DebeziumConnector {
	return &DebeziumConnector{}
}

func (dc *DebeziumConnector) SetName(name string) {
	dc.name = name
}

func (dc *DebeziumConnector) SetVersion(version string) {
	dc.version = version
}

func (dc *DebeziumConnector) Name() string {
	return dc.name + "." + dc.version
}

func (dc *DebeziumConnector) Create() (err error) {
	return
}

func (dc *DebeziumConnector) Delete() (err error) {
	return
}

func (dc *DebeziumConnector) CheckDeviation() (err error) {
	return
}

func (dc *DebeziumConnector) Exists() (exists bool, err error) {
	return
}

func (dc *DebeziumConnector) ListInstalledVersions() (versions []string, err error) {
	return
}
