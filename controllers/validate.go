package controllers

import (
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"math"
)

const countMismatchThreshold = 0.5
const idDiffMaxLength = 50

func (i *ReconcileIteration) validate() (isValid bool, mismatchRatio float64, mismatchCount int, hostCount int, err error) {
	hbiHostCount, err := i.InventoryDb.CountHosts()
	if err != nil {
		return false, -1, -1, -1, err
	}

	esCount, err := i.ESClient.CountIndex(elasticsearch.ESIndexName(i.config.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion))
	if err != nil {
		return false, -1, -1, -1, err
	}

	metrics.ESHostCount(i.Instance, esCount)

	countMismatch := utils.Abs(hbiHostCount - esCount)
	countMismatchRatio := float64(countMismatch) / math.Max(float64(hbiHostCount), 1)

	i.Log.Info("Fetched host counts", "hbi", hbiHostCount,
		"es", esCount, "countMismatchRatio", countMismatchRatio)

	// if the counts are way off don't even bother comparing ids
	if countMismatchRatio > countMismatchThreshold {
		i.Log.Info("Count mismatch ratio is above threshold, exiting early",
			"countMismatchRatio", countMismatchRatio)
		metrics.ValidationFinished(
			i.getValidationPercentageThreshold(), countMismatchRatio, countMismatch, false)
		return false, countMismatchRatio, countMismatch, esCount, nil
	}

	hbiIds, err := i.InventoryDb.GetHostIds()
	if err != nil {
		return false, -1, -1, -1, err
	}

	esIds, err := i.ESClient.GetHostIDs(elasticsearch.ESIndexName(i.config.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion))
	if err != nil {
		return false, -1, -1, -1, err
	}

	i.Log.Info("Fetched host ids")
	inHbiOnly := utils.Difference(hbiIds, esIds)
	inAppOnly := utils.Difference(esIds, hbiIds)
	mismatchCount = len(inHbiOnly) + len(inAppOnly)

	validationThresholdPercent := i.getValidationPercentageThreshold()

	idMismatchRatio := float64(mismatchCount) / math.Max(float64(len(hbiIds)), 1)
	result := (idMismatchRatio * 100) <= float64(validationThresholdPercent)

	metrics.ValidationFinished(i.getValidationPercentageThreshold(), idMismatchRatio, mismatchCount, result)
	i.Log.Info(
		"Validation results",
		"validationThresholdPercent", validationThresholdPercent,
		"idMismatchRatio", idMismatchRatio,
		// if the list is too long truncate it to first 50 ids to avoid log pollution
		"inHbiOnly", inHbiOnly[:utils.Min(idDiffMaxLength, len(inHbiOnly))],
		"inAppOnly", inAppOnly[:utils.Min(idDiffMaxLength, len(inAppOnly))],
	)
	return result, idMismatchRatio, mismatchCount, esCount, nil
}
