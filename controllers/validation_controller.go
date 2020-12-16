package controllers

import (
	"context"
	"fmt"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"k8s.io/client-go/tools/record"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ValidationReconciler struct {
	XJoinPipelineReconciler

	// if true the Reconciler will check for pipeline state deviation
	// should always be true except for tests
	CheckResourceDeviation bool
}

func (r *ValidationReconciler) setup(reqLogger logr.Logger, request ctrl.Request) (ReconcileIteration, error) {
	i, err := r.XJoinPipelineReconciler.setup(reqLogger, request)

	if err != nil || i.Instance == nil {
		return i, err
	}

	i.GetRequeueInterval = func(i *ReconcileIteration) int64 {
		return i.getValidationConfig().Interval
	}

	i.InventoryDb = database.NewBaseDatabase(&i.HBIDBParams)

	if err = i.InventoryDb.Connect(); err != nil {
		return i, err
	}

	return i, err
}

func (r *ValidationReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	reqLogger := r.Log.WithValues("Pipeline", request.Name, "Namespace", request.Namespace)

	i, err := r.setup(reqLogger, request)
	defer i.Close()

	if err != nil {
		i.error(err)
		return reconcile.Result{}, err
	}

	// nothing to validate
	if i.Instance == nil || i.Instance.GetState() == xjoin.STATE_REMOVED || i.Instance.GetState() == xjoin.STATE_NEW {
		return reconcile.Result{}, nil
	}

	reqLogger.Info("Validating XJoinPipeline")

	if r.CheckResourceDeviation {
		problem, err := i.checkForDeviation()
		if err != nil {
			i.error(err, "Error checking for state deviation")
			return reconcile.Result{}, err
		} else if problem != nil {
			i.probeStateDeviationRefresh(problem.Error())
			if err = i.Instance.TransitionToNew(); err != nil {
				return reconcile.Result{}, err
			}
			return i.updateStatusAndRequeue()
		}
	}

	isValid, mismatchRatio, mismatchCount, hostCount, err := i.validate()
	if err != nil {
		i.error(err, "Error validating pipeline")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Validation finished", "isValid", isValid)

	if isValid {
		msg := fmt.Sprintf("%v hosts (%.2f%%) do not match", mismatchCount, mismatchRatio*100)

		if i.Instance.GetState() == xjoin.STATE_INVALID {
			i.eventNormal("Valid", "Pipeline is valid again")
		}

		i.Instance.SetValid(
			metav1.ConditionTrue,
			"ValidationSucceeded",
			fmt.Sprintf("Validation succeeded - %s", msg),
			hostCount,
		)
	} else {
		msg := fmt.Sprintf("Validation failed - %v hosts (%.2f%%) do not match", mismatchCount, mismatchRatio*100)

		i.Recorder.Event(i.Instance, corev1.EventTypeWarning, "ValidationFailed", msg)
		i.Instance.SetValid(
			metav1.ConditionFalse,
			"ValidationFailed",
			msg,
			hostCount,
		)
	}

	return i.updateStatusAndRequeue()
}

func eventFilterPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.MetaOld.GetGeneration() != e.MetaNew.GetGeneration() {
				return true // Pipeline definition changed
			}

			old, ok1 := e.ObjectOld.(*xjoin.XJoinPipeline)
			new, ok2 := e.ObjectNew.(*xjoin.XJoinPipeline)

			if ok1 && ok2 && old.Status.InitialSyncInProgress == false && new.Status.InitialSyncInProgress == true {
				return true // pipeline refresh happened - validate the new pipeline
			}

			return false
		},
	}
}

func (r *ValidationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-validation").
		For(&xjoin.XJoinPipeline{}).
		WithEventFilter(eventFilterPredicate()).
		Complete(r)
}

func NewValidationReconciler(client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
	checkResourceDeviation bool,
	recorder record.EventRecorder) *ValidationReconciler {
	return &ValidationReconciler{
		XJoinPipelineReconciler: *NewXJoinReconciler(client, scheme, log, recorder),
		CheckResourceDeviation:  checkResourceDeviation,
	}
}
