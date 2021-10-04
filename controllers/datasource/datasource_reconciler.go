package datasource

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/components"
)

type ReconcileMethods struct {
	iteration XJoinDataSourceIteration
}

func NewReconcileMethods(iteration XJoinDataSourceIteration) *ReconcileMethods {
	return &ReconcileMethods{
		iteration: iteration,
	}
}

func (d *ReconcileMethods) Removed() (err error) {
	err = d.iteration.Finalize()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) New(version string) (err error) {
	err = d.iteration.CreateDataSourcePipeline(d.iteration.GetInstance().Name, version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) InitialSync() (err error) {
	err = d.iteration.ReconcilePipelines()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) Valid() (err error) {
	err = d.iteration.ReconcilePipelines()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) StartRefreshing(version string) (err error) {
	err = d.iteration.CreateDataSourcePipeline(d.iteration.GetInstance().Name, version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) Refreshing() (err error) {
	err = d.iteration.ReconcilePipelines()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) RefreshComplete() (err error) {
	err = d.iteration.DeleteDataSourcePipeline(d.iteration.GetInstance().Name, d.iteration.GetInstance().Status.ActiveVersion)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *ReconcileMethods) Scrub() (err error) {
	var validVersions []string
	if d.iteration.GetInstance().Status.ActiveVersion != "" {
		validVersions = append(validVersions, d.iteration.GetInstance().Status.ActiveVersion)
	}
	if d.iteration.GetInstance().Status.RefreshingVersion != "" {
		validVersions = append(validVersions, d.iteration.GetInstance().Status.RefreshingVersion)
	}

	custodian := components.NewCustodian(
		d.iteration.GetInstance().Kind+"."+d.iteration.GetInstance().Name, validVersions)
	custodian.AddComponent(components.NewAvroSchema("", nil))
	custodian.AddComponent(components.NewKafkaTopic())
	custodian.AddComponent(components.NewDebeziumConnector())
	err = custodian.Scrub()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}
