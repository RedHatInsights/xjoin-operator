package v1alpha1

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineState string

const validConditionType = "Valid"

const (
	STATE_NEW          PipelineState = "NEW"
	STATE_INITIAL_SYNC PipelineState = "INITIAL_SYNC"
	STATE_VALID        PipelineState = "VALID"
	STATE_INVALID      PipelineState = "INVALID"
	STATE_REMOVED      PipelineState = "REMOVED"
	STATE_UNKNOWN      PipelineState = "UNKNOWN"
)

func (instance *XJoinPipeline) GetState() PipelineState {
	switch {
	case instance.GetDeletionTimestamp() != nil:
		return STATE_REMOVED
	case instance.Status.PipelineVersion == "":
		return STATE_NEW
	case instance.IsValid():
		return STATE_VALID
	case instance.Status.InitialSyncInProgress == true:
		return STATE_INITIAL_SYNC
	case instance.GetValid() == metav1.ConditionFalse:
		return STATE_INVALID
	default:
		return STATE_UNKNOWN
	}
}

func (instance *XJoinPipeline) SetValid(status metav1.ConditionStatus, reason string, message string, hostCount int64) {
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:    validConditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	})

	switch status {
	case metav1.ConditionFalse:
		instance.Status.ValidationFailedCount++
	case metav1.ConditionUnknown:
		instance.Status.ValidationFailedCount = 0
	case metav1.ConditionTrue:
		instance.Status.ValidationFailedCount = 0
		instance.Status.InitialSyncInProgress = false
	}
}

func (instance *XJoinPipeline) ResetValid() {
	instance.SetValid(metav1.ConditionUnknown, "New", "Validation not yet run", -1)
}

func (instance *XJoinPipeline) IsValid() bool {
	return meta.IsStatusConditionPresentAndEqual(instance.Status.Conditions, validConditionType, metav1.ConditionTrue)
}

func (instance *XJoinPipeline) GetValid() metav1.ConditionStatus {
	condition := meta.FindStatusCondition(instance.Status.Conditions, validConditionType)

	if condition == nil {
		return metav1.ConditionUnknown
	}

	return condition.Status
}

func (instance *XJoinPipeline) TransitionToInitialSync(pipelineVersion string) error {
	if err := instance.assertState(STATE_INITIAL_SYNC, STATE_INITIAL_SYNC, STATE_NEW); err != nil {
		return err
	}

	instance.ResetValid()
	instance.Status.InitialSyncInProgress = true
	instance.Status.PipelineVersion = pipelineVersion

	return nil
}

func (instance *XJoinPipeline) TransitionToNew() error {
	instance.ResetValid()
	instance.Status.InitialSyncInProgress = false
	instance.Status.PipelineVersion = ""
	return nil
}

func (instance *XJoinPipeline) assertState(targetState PipelineState, validStates ...PipelineState) error {
	for _, state := range validStates {
		if instance.GetState() == state {
			return nil
		}
	}

	return fmt.Errorf("Attempted invalid state transition from %s to %s", instance.GetState(), targetState)
}

func DebeziumConnectorName(pipelineVersion string) string {
	return fmt.Sprintf("xjoin.inventory.hosts.db.%s", pipelineVersion)
}

func ESConnectorName(pipelineVersion string) string {
	return fmt.Sprintf("xjoin.inventory.hosts.es.%s", pipelineVersion)
}
