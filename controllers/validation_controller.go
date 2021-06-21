package controllers

import (
	"github.com/go-logr/logr"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
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

func (r *ValidationReconciler) setup(reqLogger logger.Log, request ctrl.Request) (ReconcileIteration, error) {
	i, err := r.XJoinPipelineReconciler.setup(reqLogger, request)

	if err != nil || i.Instance == nil {
		return i, err
	}

	if i.Instance.Spec.Pause == true {
		return i, nil
	}

	i.GetRequeueInterval = func(i *ReconcileIteration) int {
		return i.getValidationInterval()
	}

	return i, err
}

func (r *ValidationReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	reqLogger := logger.NewLogger("controller_validation", "Pipeline", request.Name, "Namespace", request.Namespace)

	i, err := r.setup(reqLogger, request)
	defer i.Close()

	if err != nil {
		i.error(err)
		return reconcile.Result{}, err
	}

	if i.Instance != nil {
		reqLogger.Debug("Instance State", "state", i.Instance.GetState())
	}

	// nothing to validate
	if i.Instance == nil ||
		i.Instance.GetState() == xjoin.STATE_REMOVED ||
		i.Instance.GetState() == xjoin.STATE_NEW ||
		i.Instance.Spec.Pause == true {

		return reconcile.Result{}, nil
	}

	reqLogger.Info("Checking for resource deviation")

	if r.CheckResourceDeviation {
		problem, err := i.checkForDeviation()
		if err != nil {
			i.error(err, "Error checking for state deviation")
			return reconcile.Result{}, err
		} else if problem != nil {
			i.probeStateDeviationRefresh(problem.Error())
			i.Instance.TransitionToNew()
			return i.updateStatusAndRequeue()
		}
	}

	reqLogger.Info("Validating XJoinPipeline",
		"LagCompensationSeconds", i.parameters.ValidationLagCompensationSeconds.Int(),
		"ValidationPeriodMinutes", i.parameters.ValidationPeriodMinutes.Int(),
		"FullValidationEnabled", i.parameters.FullValidationEnabled.Bool(),
		"FullValidationNumThreads", i.parameters.FullValidationNumThreads.Int(),
		"FullValidationChunkSize", i.parameters.FullValidationChunkSize.Int())

	isValid, err := i.validate()
	if err != nil {
		i.error(err, "Error validating pipeline")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Validation finished", "isValid", isValid)

	if isValid {
		if i.Instance.GetState() == xjoin.STATE_INVALID {
			i.eventNormal("Valid", "Pipeline is valid again")
		}
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
