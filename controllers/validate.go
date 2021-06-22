package controllers

import (
	"errors"
	"fmt"
	"github.com/go-test/deep"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
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

func (i *ReconcileIteration) validate() (isValid bool, err error) {
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

	if i.parameters.FullValidationEnabled.Bool() == true {
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

	now := time.Now().UTC()
	validationLagComp := i.parameters.ValidationLagCompensationSeconds.Int()
	endTime := now.Add(-time.Duration(validationLagComp) * time.Second)
	hostCount, err := i.InventoryDb.CountHosts(endTime)
	if err != nil {
		return
	}

	esCount, err := i.ESClient.CountIndex(i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion), endTime)
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
		i.eventNormal("CountValidationFailed", msg)
	} else {
		i.eventNormal("CountValidationPassed", msg)
	}

	metrics.CountValidationFinished(
		i.getValidationPercentageThreshold(), mismatchRatio, mismatchCount)

	return
}

func (i *ReconcileIteration) idValidation() (isValid bool, mismatchCount int, mismatchRatio float64, hbiIds []string, err error) {
	isValid = false

	now := time.Now().UTC()
	validationPeriod := i.parameters.ValidationPeriodMinutes.Int()
	validationLagComp := i.parameters.ValidationLagCompensationSeconds.Int()

	var startTime time.Time
	if i.Instance.GetState() == xjoin.STATE_INITIAL_SYNC {
		startTime = time.Unix(0, 0)
	} else {
		startTime = now.Add(-time.Duration(validationPeriod) * time.Minute)
	}
	endTime := now.Add(-time.Duration(validationLagComp) * time.Second)

	hbiIds, err = i.InventoryDb.GetHostIds(startTime, endTime)
	if err != nil {
		return
	}

	esIds, err := i.ESClient.GetHostIDs(i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion), startTime, endTime)
	if err != nil {
		return
	}

	i.Log.Info("Fetched host ids")
	inHbiOnly := utils.Difference(hbiIds, esIds)
	inAppOnly := utils.Difference(esIds, hbiIds)
	mismatchCount = len(inHbiOnly) + len(inAppOnly)

	validationThresholdPercent := i.getValidationPercentageThreshold()

	mismatchRatio = float64(mismatchCount) / math.Max(float64(len(hbiIds)), 1)
	isValid = (mismatchRatio * 100) <= float64(validationThresholdPercent)

	msg := fmt.Sprintf("%v hosts ids do not match. Number of hosts IDs retrieved: HBI: %v, ES: %v", mismatchCount, len(hbiIds), len(esIds))
	if !isValid {
		i.eventNormal("IDValidationFailed", msg)
	} else {
		i.eventNormal("IDValidationPassed", msg)
	}

	i.Log.Info(
		"ID Validation results",
		"validationThresholdPercent", i.getValidationPercentageThreshold(),
		"mismatchRatio", mismatchRatio,
		"mismatchCount", mismatchCount,
		"totalHBIHostsRetrieved", len(hbiIds),
		"totalESHostsRetrieved", len(esIds),
		// if the list is too long truncate it to first 50 ids to avoid log pollution
		"inHbiOnly", inHbiOnly[:utils.Min(idDiffMaxLength, len(inHbiOnly))],
		"inAppOnly", inAppOnly[:utils.Min(idDiffMaxLength, len(inAppOnly))],
	)

	metrics.IDValidationFinished(
		i.getValidationPercentageThreshold(), mismatchRatio, mismatchCount)

	return
}

type idDiff struct {
	id   string
	diff string
}

func (i *ReconcileIteration) validateChunk(chunk []string, allIdDiffs chan idDiff, errorsChan chan error, wg *sync.WaitGroup) {
	defer wg.Done()

	now := time.Now().UTC()
	validationLagComp := i.parameters.ValidationLagCompensationSeconds.Int()
	endTime := now.Add(-time.Duration(validationLagComp) * time.Second)

	//retrieve hosts from db and es
	esHosts, err := i.ESClient.GetHostsByIds(i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion), chunk, endTime)
	if err != nil {
		errorsChan <- err
		return
	}

	hbiHosts, err := i.InventoryDb.GetHostsByIds(chunk, endTime)
	if err != nil {
		errorsChan <- err
		return
	}

	deep.MaxDiff = len(chunk) * 100
	diffs := deep.Equal(hbiHosts, esHosts)

	//build the change object for logging
	for _, diff := range diffs {
		idxStr := diff[strings.Index(diff, "[")+1 : strings.Index(diff, "]")]

		idx, err := strconv.ParseInt(idxStr, 10, 64)
		if err != nil {
			errorsChan <- err
			return
		}
		id := chunk[idx]
		allIdDiffs <- idDiff{id: id, diff: diff}
	}

	return
}

func (i *ReconcileIteration) fullValidation(ids []string) (isValid bool, mismatchCount int, mismatchRatio float64, err error) {
	allIdDiffs := make(chan idDiff, len(ids)*100)
	errorsChan := make(chan error, len(ids))
	numThreads := 0
	wg := new(sync.WaitGroup)

	chunkSize := i.parameters.FullValidationChunkSize.Int()
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
		go i.validateChunk(chunk, allIdDiffs, errorsChan, wg)

		if numThreads == i.parameters.FullValidationNumThreads.Int() || j == numChunks-1 {
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

		return false, -1, -1, errors.New("error during full validation")
	}

	//group diffs by id for counting mismatched systems
	diffsById := make(map[string][]string)
	for d := range allIdDiffs {
		diffsById[d.id] = append(diffsById[d.id], d.diff)
	}

	//determine if the data is valid within the threshold
	mismatchCount = len(diffsById)
	mismatchRatio = float64(mismatchCount) / math.Max(float64(len(ids)), 1)
	isValid = (mismatchRatio * 100) <= float64(i.getValidationPercentageThreshold())
	msg := fmt.Sprintf("%v hosts do not match. %v hosts validated.", mismatchCount, len(ids))

	if !isValid {
		isValid = false
		i.eventNormal("FullValidationFailed", msg)
	} else {
		i.eventNormal("FullValidationPassed", msg)
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
		"validationThresholdPercent", i.getValidationPercentageThreshold(),
		"mismatchRatio", mismatchRatio,
		"mismatchCount", mismatchCount,
		"mismatchedHosts", diffsToLog,
	)

	metrics.FullValidationFinished(
		i.getValidationPercentageThreshold(), mismatchRatio, mismatchCount)

	return
}
