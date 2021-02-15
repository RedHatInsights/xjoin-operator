package controllers

import "github.com/redhatinsights/xjoin-operator/controllers/metrics"

func (i *ReconcileIteration) probeStartingInitialSync() {
	i.Log.Info("New pipeline version", "version", i.Instance.Status.PipelineVersion)
	//i.eventNormal("InitialSync", "Starting data synchronization to %s", i.instance.Status.TableName)
	i.Log.Info("Transitioning to InitialSync")
}

func (i *ReconcileIteration) probeStateDeviationRefresh(reason string) {
	i.Log.Info("Refreshing pipeline due to state deviation", "reason", reason)
	metrics.PipelineRefreshed("deviation")
	i.eventWarning("Refreshing", "Refreshing pipeline due to state deviation: %s", reason)
}

func (i *ReconcileIteration) probePipelineDidNotBecomeValid() {
	i.Log.Info("Pipeline failed to become valid. Refreshing.")
	i.eventWarning("Refreshing", "Pipeline failed to become valid within the given threshold")
	metrics.PipelineRefreshed("invalid")
}
