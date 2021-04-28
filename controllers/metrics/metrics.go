package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	hostCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_hosts_total",
		Help: "Total number of hosts in the given ElasticSearch Index",
	}, []string{})

	countInconsistencyRatio = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_count_inconsistency_ratio",
		Help: "The ratio of missing hosts between HBI and ElasticSearch",
	}, []string{})

	countInconsistencyAbsolute = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_count_inconsistency_total",
		Help: "The total number of hosts that are missing in ElasticSearch",
	}, []string{})

	idInconsistencyRatio = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_id_inconsistency_ratio",
		Help: "The ratio of inconsistent host IDs between HBI and ElasticSearch",
	}, []string{})

	idInconsistencyAbsolute = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_id_inconsistency_total",
		Help: "The total number of host IDs that are inconsistent between HBI and ElasticSearch",
	}, []string{})

	fullInconsistencyRatio = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_full_inconsistency_ratio",
		Help: "The ratio of hosts with invalid data between HBI and ElasticSearch",
	}, []string{})

	fullInconsistencyAbsolute = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_full_inconsistency_total",
		Help: "The total number hosts with invalid data between HBI and ElasticSearch",
	}, []string{})

	inconsistencyThreshold = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_inconsistency_threshold",
		Help: "The threshold of inconsistency below which the pipeline is considered valid",
	}, []string{})

	validationFailedCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "xjoin_validation_failed_total",
		Help: "The number of validation iterations that failed",
	}, []string{})

	refreshCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "xjoin_refresh_total",
		Help: "The number of times this pipeline has been refreshed",
	}, []string{"reason"})
)

type RefreshReason string

const (
	RefreshInvalidPipeline RefreshReason = "invalid"
	RefreshStateDeviation  RefreshReason = "deviation"
)

func Init() {
	metrics.Registry.MustRegister(
		hostCount,
		countInconsistencyRatio,
		countInconsistencyAbsolute,
		idInconsistencyRatio,
		idInconsistencyAbsolute,
		fullInconsistencyRatio,
		fullInconsistencyAbsolute,
		inconsistencyThreshold,
		validationFailedCount,
		refreshCount)
}

func InitLabels() {
	hostCount.WithLabelValues()
	inconsistencyThreshold.WithLabelValues()
	countInconsistencyRatio.WithLabelValues()
	countInconsistencyAbsolute.WithLabelValues()
	idInconsistencyRatio.WithLabelValues()
	idInconsistencyAbsolute.WithLabelValues()
	fullInconsistencyRatio.WithLabelValues()
	fullInconsistencyAbsolute.WithLabelValues()
	validationFailedCount.WithLabelValues()
	refreshCount.WithLabelValues(string(RefreshInvalidPipeline))
	refreshCount.WithLabelValues(string(RefreshStateDeviation))
}

func PipelineRefreshed(reason RefreshReason) {
	refreshCount.WithLabelValues(string(reason)).Inc()
}

func ESHostCount(value int) {
	hostCount.WithLabelValues().Set(float64(value))
}

func FullValidationFinished(threshold int, ratio float64, inconsistentTotal int) {
	inconsistencyThreshold.WithLabelValues().Set(float64(threshold) / 100)
	fullInconsistencyRatio.WithLabelValues().Set(ratio)
	fullInconsistencyAbsolute.WithLabelValues().Set(float64(inconsistentTotal))
}

func IDValidationFinished(threshold int, ratio float64, inconsistentTotal int) {
	inconsistencyThreshold.WithLabelValues().Set(float64(threshold) / 100)
	idInconsistencyRatio.WithLabelValues().Set(ratio)
	idInconsistencyAbsolute.WithLabelValues().Set(float64(inconsistentTotal))
}

func CountValidationFinished(threshold int, ratio float64, inconsistentTotal int) {
	inconsistencyThreshold.WithLabelValues().Set(float64(threshold) / 100)
	countInconsistencyRatio.WithLabelValues().Set(ratio)
	countInconsistencyAbsolute.WithLabelValues().Set(float64(inconsistentTotal))
}

func ValidationFinished(isValid bool) {
	if !isValid {
		validationFailedCount.WithLabelValues().Inc()
	}
}
