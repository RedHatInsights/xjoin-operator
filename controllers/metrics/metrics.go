package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"strconv"
	"strings"
)

var log = logger.NewLogger("metrics")

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

	connectorTaskRestartCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "xjoin_connector_task_restart_total",
		Help: "The number of times a connector task has been restarted",
	}, []string{"connector"})

	connectRestartCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "xjoin_connect_restart_total",
		Help: "The number of times Kafka Connect has been restarted",
	}, []string{})

	staleResourceCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_stale_resource_count",
		Help: "The number of stale resources found during each reconcile loop",
	}, []string{"count"})
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
		refreshCount,
		connectorTaskRestartCount,
		connectRestartCount,
		staleResourceCount)
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
	connectRestartCount.WithLabelValues()
}

func StaleResourceCount(count int, resources []string) {
	staleResourceCount.
		With(prometheus.Labels{
			"count": strconv.Itoa(count)}).
		Set(float64(count))

	if len(resources) > 0 {
		log.Info("Stale resources found", "resources", strings.Join(resources, ","))
	}
}

func ConnectRestarted() {
	connectRestartCount.WithLabelValues().Inc()
}

func ConnectorTaskRestarted(connector string) {
	connectorTaskRestartCount.With(prometheus.Labels{"connector": connector}).Inc()
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
