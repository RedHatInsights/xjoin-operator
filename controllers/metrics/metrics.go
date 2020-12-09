package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	hostCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_hosts_total",
		Help: "Total number of hosts in the given table in the application database",
	}, []string{"app"})

	inconsistencyRatio = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_inconsistency_ratio",
		Help: "The ratio of inconsistency of data between the source database and the application replica",
	}, []string{"app"})

	inconsistencyAbsolute = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_inconsistency_total",
		Help: "The total number of hosts that are not consistent with the origin",
	}, []string{"app"})

	inconsistencyThreshold = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_inconsistency_threshold",
		Help: "The threshold of inconsistency below which the pipeline is considered valid",
	}, []string{"app"})

	validationFailedCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "xjoin_validation_failed_total",
		Help: "The number of validation iterations that failed",
	}, []string{"app"})

	refreshCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "xjoin_refresh_total",
		Help: "The number of times this pipeline has been refreshed",
	}, []string{"app", "reason"})
)

type RefreshReason string

const (
	REFRESH_INVALID_PIPELINE RefreshReason = "invalid"
	REFRESH_STATE_DEVIATION  RefreshReason = "deviation"
)

func Init() {
	metrics.Registry.MustRegister(hostCount, inconsistencyRatio, inconsistencyAbsolute, inconsistencyThreshold, validationFailedCount, refreshCount)
}

func InitLabels(instance *xjoin.XJoinPipeline) {
}

func PipelineRefreshed(instance *xjoin.XJoinPipeline, reason RefreshReason) {
	refreshCount.WithLabelValues(string(reason)).Inc()
}
