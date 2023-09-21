package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
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
	}, []string{})
)

//v2 metrics
const XJoinIndexPipelineLabel = "xjoin_index_pipeline"
const XJoinIndexLabel = "xjoin_index"
const XJoinDatasourceLabel = "xjoin_datasource"

var (
	indexDocumentCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_index_document_total",
		Help: "Total number of documents in the given ElasticSearch Index",
	}, []string{XJoinIndexPipelineLabel})

	countInconsistencyRatioV2 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_count_inconsistency_ratio_v2",
		Help: "The ratio of missing records between a Database and ElasticSearch",
	}, []string{XJoinIndexPipelineLabel})

	countInconsistencyAbsoluteV2 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_count_inconsistency_total_v2",
		Help: "The total number of records that are missing in ElasticSearch",
	}, []string{XJoinIndexPipelineLabel})

	idInconsistencyRatioV2 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_id_inconsistency_ratio_v2",
		Help: "The ratio of inconsistent record IDs between the Database and ElasticSearch",
	}, []string{XJoinIndexPipelineLabel})

	idInconsistencyAbsoluteV2 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_id_inconsistency_total_v2",
		Help: "The total number of record IDs that are inconsistent between the Database and ElasticSearch",
	}, []string{XJoinIndexPipelineLabel})

	contentInconsistencyRatioV2 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_full_inconsistency_ratio_v2",
		Help: "The ratio of records with invalid data between the Database and ElasticSearch",
	}, []string{XJoinIndexPipelineLabel})

	contentInconsistencyAbsoluteV2 = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_full_inconsistency_total_v2",
		Help: "The total number records with invalid data between the Database and ElasticSearch",
	}, []string{XJoinIndexPipelineLabel})

	validationFailedCountV2 = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "xjoin_validation_failed_total_v2",
		Help: "The number of validation iterations that failed",
	}, []string{XJoinIndexPipelineLabel})

	amountOfIdsValidated = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_amount_of_ids_validated",
		Help: "The number of ids validated",
	}, []string{XJoinIndexPipelineLabel})

	amountOfContentsValidated = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xjoin_amount_of_contents_validated",
		Help: "The number of record contents validated",
	}, []string{XJoinIndexPipelineLabel})

	indexRefreshCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "xjoin_index_refresh_count",
		Help: "The number of times an index has been refreshed",
	}, []string{XJoinIndexLabel})

	datasourceRefreshCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "xjoin_datasource_refresh_count",
		Help: "The number of times a datasource has been refreshed",
	}, []string{XJoinDatasourceLabel})

	validationPodFailedCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "xjoin_validation_pod_failed_count",
		Help: "The number of times a validation pod failed for an indexpipeline",
	}, []string{XJoinDatasourceLabel})
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
		staleResourceCount,

		//v2 resources
		indexDocumentCount,
		countInconsistencyRatioV2,
		countInconsistencyAbsoluteV2,
		idInconsistencyRatioV2,
		idInconsistencyAbsoluteV2,
		contentInconsistencyRatioV2,
		contentInconsistencyAbsoluteV2,
		validationFailedCountV2,
		indexRefreshCount,
		datasourceRefreshCount,
		validationPodFailedCount,
		amountOfIdsValidated,
		amountOfContentsValidated)
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

func InitIndexValidatorLabels(indexPipeline string) {
	indexDocumentCount.WithLabelValues(indexPipeline)
	countInconsistencyRatioV2.WithLabelValues(indexPipeline)
	countInconsistencyAbsoluteV2.WithLabelValues(indexPipeline)
	idInconsistencyRatioV2.WithLabelValues(indexPipeline)
	idInconsistencyAbsoluteV2.WithLabelValues(indexPipeline)
	contentInconsistencyRatioV2.WithLabelValues(indexPipeline)
	contentInconsistencyAbsoluteV2.WithLabelValues(indexPipeline)
	validationFailedCountV2.WithLabelValues(indexPipeline)
	validationPodFailedCount.WithLabelValues(indexPipeline)
	amountOfIdsValidated.WithLabelValues(indexPipeline)
	amountOfContentsValidated.WithLabelValues(indexPipeline)
}

func InitIndexLabels(index string) {
	indexRefreshCount.WithLabelValues(index)
}

func InitDatasourceLabels(datasource string) {
	datasourceRefreshCount.WithLabelValues(datasource)
}

func StaleResourceCount(resources []string) {
	staleResourceCount.
		WithLabelValues().
		Set(float64(len(resources)))

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

func ValidationFinishedV2(pipeline string, response validation.ValidationResponse) {
	if response.Result == validation.ValidationInvalid {
		validationFailedCountV2.With(prometheus.Labels{XJoinIndexPipelineLabel: pipeline}).Inc()
	}

	indexDocumentCount.WithLabelValues(pipeline).Set(float64(response.Details.Counts.RecordCountInElasticsearch))

	//counts
	if response.Details.Counts.InconsistencyRatio != "" {
		countInconsistencyRatioValue, err := strconv.ParseFloat(response.Details.Counts.InconsistencyRatio, 64)
		if err != nil {
			log.Error(err, "unable to parse count inconsistency ratio metric")
		}
		countInconsistencyRatioV2.WithLabelValues(pipeline).Set(countInconsistencyRatioValue)
	} else {
		countInconsistencyRatioV2.WithLabelValues(pipeline).Set(0)
	}
	countInconsistencyAbsoluteV2.WithLabelValues(pipeline).Set(float64(response.Details.Counts.InconsistencyAbsolute))

	//ids
	if response.Details.IDs.InconsistencyRatio != "" {
		idInconsistencyRatioValue, err := strconv.ParseFloat(response.Details.IDs.InconsistencyRatio, 64)
		if err != nil {
			log.Error(err, "unable to parse id inconsistency ratio metric")
		}
		idInconsistencyRatioV2.WithLabelValues(pipeline).Set(idInconsistencyRatioValue)
	} else {
		idInconsistencyRatioV2.WithLabelValues(pipeline).Set(0)
	}
	idInconsistencyAbsoluteV2.WithLabelValues(pipeline).Set(float64(response.Details.IDs.InconsistencyAbsolute))
	amountOfIdsValidated.WithLabelValues(pipeline).Set(float64(response.Details.IDs.AmountValidated))

	//content
	if response.Details.Content.InconsistencyRatio != "" {
		contentInconsistencyRatioValue, err := strconv.ParseFloat(response.Details.Content.InconsistencyRatio, 64)
		if err != nil {
			log.Error(err, "unable to parse content inconsistency ratio metric")
		}
		contentInconsistencyRatioV2.WithLabelValues(pipeline).Set(contentInconsistencyRatioValue)
	} else {
		contentInconsistencyRatioV2.WithLabelValues(pipeline).Set(0)
	}
	contentInconsistencyAbsoluteV2.WithLabelValues(pipeline).Set(float64(response.Details.Content.InconsistencyAbsolute))
	amountOfContentsValidated.WithLabelValues(pipeline).Set(float64(response.Details.Content.AmountValidated))
}

func ValidationPodFailed(pipeline string) {
	validationPodFailedCount.WithLabelValues(pipeline).Inc()
}

func IndexRefreshing(index string) {
	indexRefreshCount.WithLabelValues(index).Inc()
}

func DatasourceRefreshing(datasource string) {
	datasourceRefreshCount.WithLabelValues(datasource).Inc()
}
