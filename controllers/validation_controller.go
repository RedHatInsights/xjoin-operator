package controllers

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	. "github.com/redhatinsights/xjoin-operator/controllers/pipeline"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type ValidationReconciler struct {
	XJoinPipelineReconciler

	// if true the Reconciler will check for pipeline state deviation
	// should always be true except for tests
	CheckResourceDeviation bool
}

func (r *ValidationReconciler) setup(reqLogger logger.Log, request ctrl.Request, ctx context.Context) (ReconcileIteration, error) {
	i, err := r.XJoinPipelineReconciler.setup(reqLogger, request, ctx)

	if err != nil || i.Instance == nil {
		return i, err
	}

	if i.Instance.Spec.Pause {
		return i, nil
	}

	i.GetRequeueInterval = func(i *ReconcileIteration) int {
		return i.GetValidationInterval()
	}

	return i, err
}

func (r *ValidationReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := logger.NewLogger("controller_validation", "Pipeline", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling Validation")

	i, err := r.setup(reqLogger, request, ctx)

	if err != nil {
		i.Error(err)
		return reconcile.Result{}, err
	}

	defer i.Close()

	if i.Instance != nil {
		reqLogger.Debug("Instance State", "state", i.Instance.GetState())
	}

	// nothing to validate
	if i.Instance == nil ||
		i.Instance.GetState() == xjoin.STATE_REMOVED ||
		i.Instance.GetState() == xjoin.STATE_NEW ||
		i.Instance.Spec.Pause {

		return reconcile.Result{}, nil
	}

	reqLogger.Info("Checking for resource deviation")

	if r.CheckResourceDeviation {
		reqLogger.Info("Really checking for deviation")
		problem, err := i.CheckForDeviation()
		if err != nil {
			i.Error(err, "Error checking for state deviation")
			return reconcile.Result{}, errors.Wrap(err, 0)
		} else if problem != nil {
			reqLogger.Info("Problem checking for deviation")
			i.ProbeStateDeviationRefresh(problem.Error())
			reqLogger.Info("Transitioning to new state")
			i.Instance.TransitionToNew()
			return i.UpdateStatusAndRequeue()
		}
	}

	reqLogger.Info("Validating XJoinPipeline",
		"LagCompensationSeconds", i.Parameters.ValidationLagCompensationSeconds.Int(),
		"ValidationPeriodMinutes", i.Parameters.ValidationPeriodMinutes.Int(),
		"FullValidationEnabled", i.Parameters.FullValidationEnabled.Bool(),
		"FullValidationNumThreads", i.Parameters.FullValidationNumThreads.Int(),
		"FullValidationChunkSize", i.Parameters.FullValidationChunkSize.Int())

	isValid, err := i.Validate()
	if err != nil {
		i.Error(err, "Error validating pipeline")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Validation finished", "isValid", isValid)

	if isValid {
		if i.Instance.GetState() == xjoin.STATE_INVALID {
			i.EventNormal("Valid", "Pipeline is valid again")
		}
	}

	return i.UpdateStatusAndRequeue()
}

func eventFilterPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() {
				return true // Pipeline definition changed
			}

			oldPipeline, ok1 := e.ObjectOld.(*xjoin.XJoinPipeline)
			newPipeline, ok2 := e.ObjectNew.(*xjoin.XJoinPipeline)

			if ok1 && ok2 && !oldPipeline.Status.InitialSyncInProgress && newPipeline.Status.InitialSyncInProgress {
				return true // pipeline refresh happened - validate the new pipeline
			}

			return false
		},
	}
}

func (r *ValidationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	logConstructor := func(r *reconcile.Request) logr.Logger {
		return mgr.GetLogger()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-validation").
		For(&xjoin.XJoinPipeline{}).
		WithEventFilter(eventFilterPredicate()).
		WithLogConstructor(logConstructor).
		WithOptions(controller.Options{
			LogConstructor: logConstructor,
			RateLimiter:    workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, 1*time.Minute),
		}).
		Complete(r)
}

func NewValidationReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
	checkResourceDeviation bool,
	recorder record.EventRecorder,
	namespace string,
	isTest bool) *ValidationReconciler {

	return &ValidationReconciler{
		XJoinPipelineReconciler: *NewXJoinReconciler(client, scheme, log, recorder, namespace, isTest),
		CheckResourceDeviation:  checkResourceDeviation,
	}
}
