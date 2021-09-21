package pipeline

import (
	"errors"
	"fmt"
	"github.com/go-test/deep"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/data"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const countMismatchThreshold = 0.5
const idDiffMaxLength = 50

func (i *ReconcileIteration) Validate() (isValid bool, err error) {
	isValid, countMismatchCount, countMismatchRatio, err := i.countValidation()
	if err != nil || !isValid {
		metrics.ValidationFinished(isValid)
		i.Instance.SetValid(
			metav1.ConditionFalse,
			"ValidationFailed",
			fmt.Sprintf("Count validation failed - %v hosts (%.2f%%) do not match", countMismatchCount, countMismatchRatio*100))
		return
	}

	isValid, idMismatchCount, idMismatchRatio, hbiIds, err := i.idValidation()
	if err != nil || !isValid {
		metrics.ValidationFinished(isValid)
		i.Instance.SetValid(
			metav1.ConditionFalse,
			"ValidationFailed",
			fmt.Sprintf("ID validation failed - %v hosts (%.2f%%) do not match", idMismatchCount, idMismatchRatio*100))
		return
	}

	fullMismatchCount := 0
	fullMismatchRatio := 0.0

	if i.Parameters.FullValidationEnabled.Bool() == true {
		isValid, fullMismatchCount, fullMismatchRatio, err = i.fullValidation(hbiIds)
		if err != nil || !isValid {
			metrics.ValidationFinished(isValid)
			i.Instance.SetValid(
				metav1.ConditionFalse,
				"ValidationFailed",
				fmt.Sprintf("Full validation failed - %v hosts (%.2f%%) do not match", fullMismatchCount, fullMismatchRatio*100))
			return
		}
	}

	i.Instance.SetValid(
		metav1.ConditionTrue,
		"ValidationSucceeded",
		fmt.Sprintf("Validation succeeded - %v hosts IDs (%.2f%%) do not match, and %v (%.2f%%) hosts have inconsistent data.",
			idMismatchCount, idMismatchRatio*100, fullMismatchCount, fullMismatchRatio*100))

	return
}

func (i *ReconcileIteration) countValidation() (isValid bool, mismatchCount int, mismatchRatio float64, err error) {
	isValid = false

	hostCount, err := i.InventoryDb.CountHosts()
	if err != nil {
		return
	}

	esCount, err := i.ESClient.CountIndex(i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion))
	if err != nil {
		return
	}

	metrics.ESHostCount(esCount)

	mismatchCount = utils.Abs(hostCount - esCount)
	mismatchRatio = float64(mismatchCount) / math.Max(float64(hostCount), 1)

	i.Log.Info("Fetched host counts", "hbi", hostCount,
		"es", esCount, "mismatchRatio", mismatchRatio)

	if mismatchRatio < countMismatchThreshold {
		isValid = true
	}

	msg := fmt.Sprintf(
		"Results: mismatchRatio: %v, esCount: %v, hbiCount: %v",
		mismatchRatio,
		esCount,
		hostCount)
	if !isValid {
		i.EventNormal("CountValidationFailed", msg)
	} else {
		i.EventNormal("CountValidationPassed", msg)
	}

	metrics.CountValidationFinished(
		i.GetValidationPercentageThreshold(), mismatchRatio, mismatchCount)

	return
}

func (i ReconcileIteration) validateIdChunk(hbiIds []string, esIds []string) (mismatchCount int, inHbiOnly []string, inAppOnly []string) {
	i.Log.Info("Fetched host ids")
	inHbiOnly = utils.Difference(hbiIds, esIds)
	inAppOnly = utils.Difference(esIds, hbiIds)
	mismatchCount = len(inHbiOnly) + len(inAppOnly)

	return mismatchCount, inHbiOnly, inAppOnly
}

func (i *ReconcileIteration) idValidation() (isValid bool, mismatchCount int, mismatchRatio float64, hbiIds []string, err error) {
	isValid = false

	now := time.Now().UTC()
	validationPeriod := i.Parameters.ValidationPeriodMinutes.Int()
	validationLagComp := i.Parameters.ValidationLagCompensationSeconds.Int()

	var startTime time.Time
	if i.Instance.GetState() == xjoin.STATE_INITIAL_SYNC {
		startTime = time.Unix(86400, 0) //24 hours since epoch
	} else {
		startTime = now.Add(-time.Duration(validationPeriod) * time.Minute)
	}
	endTime := now.Add(-time.Duration(validationLagComp) * time.Second)

	//validate chunk between startTime and endTime
	hbiIds, err = i.InventoryDb.GetHostIdsByModifiedOn(startTime, endTime)
	if err != nil {
		return
	}

	esIds, err := i.ESClient.GetHostIDsByModifiedOn(i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion), startTime, endTime)
	if err != nil {
		return
	}

	mismatchCount, inHbiOnly, inAppOnly := i.validateIdChunk(hbiIds, esIds)

	i.Log.Info(
		"ID Validation results before lag compensation",
		"validationThresholdPercent", i.GetValidationPercentageThreshold(),
		"mismatchRatio", mismatchRatio,
		"mismatchCount", mismatchCount,
		"totalHBIHostsRetrieved", len(hbiIds),
		"totalESHostsRetrieved", len(esIds),
		// if the list is too long truncate it to first 50 ids to avoid log pollution
		"inHbiOnly", inHbiOnly[:utils.Min(idDiffMaxLength, len(inHbiOnly))],
		"inAppOnly", inAppOnly[:utils.Min(idDiffMaxLength, len(inAppOnly))],
	)

	//re-validate any mismatched hosts to check if they were invalid due to lag
	//this can happen when the modified_on filter excludes hosts updated between retrieving hosts from the DB/ES
	if mismatchCount > 0 {
		mismatchedIds := append(hbiIds, esIds...)
		hbiIds, err = i.InventoryDb.GetHostIdsByIdList(mismatchedIds)
		if err != nil {
			return
		}

		esIds, err = i.ESClient.GetHostIDsByIdList(i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion), mismatchedIds)
		if err != nil {
			return
		}

		mismatchCount, inHbiOnly, inAppOnly = i.validateIdChunk(hbiIds, esIds)
	}

	validationThresholdPercent := i.GetValidationPercentageThreshold()
	mismatchRatio = float64(mismatchCount) / math.Max(float64(len(hbiIds)), 1)
	isValid = (mismatchRatio * 100) <= float64(validationThresholdPercent)

	msg := fmt.Sprintf("%v hosts ids do not match. Number of hosts IDs retrieved: HBI: %v, ES: %v", mismatchCount, len(hbiIds), len(esIds))
	if !isValid {
		i.EventNormal("IDValidationFailed", msg)
	} else {
		i.EventNormal("IDValidationPassed", msg)
	}

	i.Log.Info(
		"ID Validation results",
		"validationThresholdPercent", i.GetValidationPercentageThreshold(),
		"mismatchRatio", mismatchRatio,
		"mismatchCount", mismatchCount,
		"totalHBIHostsRetrieved", len(hbiIds),
		"totalESHostsRetrieved", len(esIds),
		// if the list is too long truncate it to first 50 ids to avoid log pollution
		"inHbiOnly", inHbiOnly[:utils.Min(idDiffMaxLength, len(inHbiOnly))],
		"inAppOnly", inAppOnly[:utils.Min(idDiffMaxLength, len(inAppOnly))],
	)

	metrics.IDValidationFinished(
		i.GetValidationPercentageThreshold(), mismatchRatio, mismatchCount)

	return
}

type idDiff struct {
	id   string
	diff string
}

func (i *ReconcileIteration) validateFullChunkSync(chunk []string) (allIdDiffs []idDiff, err error) {
	//retrieve hosts from db and es
	esHosts, err := i.ESClient.GetHostsByIds(i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion), chunk)
	if err != nil {
		return
	}
	if esHosts == nil {
		esHosts = make([]data.Host, 0)
	}

	hbiHosts, err := i.InventoryDb.GetHostsByIds(chunk)
	if err != nil {
		return
	}
	if hbiHosts == nil {
		hbiHosts = make([]data.Host, 0)
	}

	deep.MaxDiff = len(chunk) * 100
	diffs := deep.Equal(hbiHosts, esHosts)

	//build the change object for logging
	for _, diff := range diffs {
		idxStr := diff[strings.Index(diff, "[")+1 : strings.Index(diff, "]")]

		var idx int64
		idx, err = strconv.ParseInt(idxStr, 10, 64)
		if err != nil {
			return
		}
		id := chunk[idx]
		allIdDiffs = append(allIdDiffs, idDiff{id: id, diff: diff})
	}

	return
}

func (i *ReconcileIteration) validateFullChunkAsync(chunk []string, allIdDiffs chan idDiff, errorsChan chan error, wg *sync.WaitGroup) {
	defer wg.Done()

	diffs, err := i.validateFullChunkSync(chunk)
	if err != nil {
		errorsChan <- err
		return
	}

	for _, diff := range diffs {
		allIdDiffs <- diff
	}

	return
}

func (i *ReconcileIteration) fullValidation(ids []string) (isValid bool, mismatchCount int, mismatchRatio float64, err error) {
	allIdDiffs := make(chan idDiff, len(ids)*100)
	errorsChan := make(chan error, len(ids))
	numThreads := 0
	wg := new(sync.WaitGroup)

	chunkSize := i.Parameters.FullValidationChunkSize.Int()
	var numChunks = int(math.Ceil(float64(len(ids)) / float64(chunkSize)))

	i.Log.Info("Full Validation Start: " + time.Now().String())

	for j := 0; j < numChunks; j++ {
		//determine which chunk of systems to validate
		start := j * chunkSize
		var end int
		if j == numChunks-1 && len(ids)%chunkSize > 0 {
			end = start + (len(ids) % chunkSize)
		} else {
			end = start + chunkSize
		}
		chunk := ids[start:end]

		//validate chunks in parallel
		wg.Add(1)
		numThreads += 1
		go i.validateFullChunkAsync(chunk, allIdDiffs, errorsChan, wg)

		if numThreads == i.Parameters.FullValidationNumThreads.Int() || j == numChunks-1 {
			wg.Wait()
			numThreads = 0
		}
	}

	i.Log.Info("Full Validation End: " + time.Now().String())

	close(allIdDiffs)
	close(errorsChan)

	if len(errorsChan) > 0 {
		for e := range errorsChan {
			i.Log.Error(e, "Error during full validation")
		}

		return false, -1, -1, errors.New("Error during full validation")
	}

	//double check mismatched hosts to account for lag
	var mismatchedIds []string
	for d := range allIdDiffs {
		mismatchedIds = append(mismatchedIds, d.id)
	}

	diffsById := make(map[string][]string)
	if len(mismatchedIds) > 0 {
		var diffs []idDiff
		diffs, err = i.validateFullChunkSync(mismatchedIds)
		if err != nil {
			return
		}

		//group diffs by id for counting mismatched systems
		for _, d := range diffs {
			diffsById[d.id] = append(diffsById[d.id], d.diff)
		}
	}

	//determine if the data is valid within the threshold
	mismatchCount = len(diffsById)
	mismatchRatio = float64(mismatchCount) / math.Max(float64(len(ids)), 1)
	isValid = (mismatchRatio * 100) <= float64(i.GetValidationPercentageThreshold())
	msg := fmt.Sprintf("%v hosts do not match. %v hosts validated.", mismatchCount, len(ids))

	if !isValid {
		isValid = false
		i.EventNormal("FullValidationFailed", msg)
	} else {
		i.EventNormal("FullValidationPassed", msg)
		isValid = true
	}

	//log at most 50 invalid systems
	diffsToLog := make(map[string][]string)
	idx := 0
	for key, val := range diffsById {
		if idx > 50 {
			break
		}
		diffsToLog[key] = val
		idx++
	}

	i.Log.Info(
		"Full Validation results",
		"validationThresholdPercent", i.GetValidationPercentageThreshold(),
		"mismatchRatio", mismatchRatio,
		"mismatchCount", mismatchCount,
		"mismatchedHosts", diffsToLog,
	)

	metrics.FullValidationFinished(
		i.GetValidationPercentageThreshold(), mismatchRatio, mismatchCount)

	return
}
