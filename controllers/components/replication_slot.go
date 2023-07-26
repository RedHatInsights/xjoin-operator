package components

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"strings"
)

type ReplicationSlot struct {
	name      string
	version   string
	Namespace string
	events    events.Events
	log       logger.Log
	Database  *database.Database
}

func (dc *ReplicationSlot) SetLogger(log logger.Log) {
	dc.log = log
}

func (dc *ReplicationSlot) SetName(kind string, name string) {
	dc.name = strings.ToLower(kind + "_" + name)
}

func (dc *ReplicationSlot) SetVersion(version string) {
	dc.version = version
}

func (dc *ReplicationSlot) Name() string {
	return strings.ToLower(strings.ReplaceAll(dc.name, ".", "_")) + "_" + dc.version
}

func (dc *ReplicationSlot) Delete() (err error) {
	err = dc.Database.RemoveReplicationSlot(dc.Name())
	if err != nil {
		dc.events.Warning("DeleteReplicationSlot",
			"Unable to delete ReplicationSlot %s", dc.Name())
		return errors.Wrap(err, 0)
	}
	dc.events.Normal("DeleteReplicationSlot",
		"ReplicationSlot %s was successfully deleted", dc.Name())
	return nil
}

func (dc *ReplicationSlot) Exists() (exists bool, err error) {
	slots, err := dc.Database.ListReplicationSlots()
	if err != nil {
		dc.events.Warning("ReplicationSlotExists",
			"Unable to check if ReplicationSlot %s exists", dc.Name())
		return false, errors.Wrap(err, 0)
	}

	exists = false
	for _, slot := range slots {
		if slot == dc.Name() {
			exists = true
		}
	}

	return exists, nil
}

func (dc *ReplicationSlot) ListInstalledVersions() (versions []string, err error) {
	slots, err := dc.Database.ListReplicationSlots()
	if err != nil {
		dc.events.Warning("ReplicationSlotListInstalledVersions",
			"Unable to list installed versions for ReplicationSlot %s", dc.Name())
		return nil, errors.Wrap(err, 0)
	}

	for _, slot := range slots {
		if strings.Index(slot, dc.name) == 0 {
			version := strings.Split(slot, dc.name)[1]
			versionPieces := strings.Split(version, "_")
			versions = append(versions, versionPieces[len(versionPieces)-1])
		}
	}
	return
}

func (dc *ReplicationSlot) Create() (err error) {
	return nil //replication slots are created by the debezium connector
}

func (dc *ReplicationSlot) CheckDeviation() (problem, err error) {
	return nil, nil //replication slots are managed by the debezium connector
}

func (dc *ReplicationSlot) Reconcile() (err error) {
	return nil
}

func (dc *ReplicationSlot) SetEvents(e events.Events) {
	dc.events = e
}
