package pipeline

import "github.com/redhatinsights/xjoin-operator/controllers/metrics"

func (i *ReconcileIteration) ProbeStartingInitialSync() {
	i.Log.Info("New pipeline version", "version", i.Instance.Status.PipelineVersion)
	// i.EventNormal("InitialSync", "Starting data synchronization to %s", i.instance.Status.TableName)
	i.Log.Info("Transitioning to InitialSync")
}

func (i *ReconcileIteration) ProbeStateDeviationRefresh(reason string) {
	i.Log.Info("Refreshing pipeline due to state deviation", "reason", reason)
	metrics.PipelineRefreshed("deviation")
	i.EventWarning("Refreshing", "Refreshing pipeline due to state deviation: %s", reason)
}

func (i *ReconcileIteration) ProbePipelineDidNotBecomeValid() {
	i.Log.Info("Pipeline failed to become valid. Refreshing.")
	i.EventWarning("Refreshing", "Pipeline failed to become valid within the given threshold")
	metrics.PipelineRefreshed("invalid")
}
