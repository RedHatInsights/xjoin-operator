package controllers

import (
	"context"
	"github.com/go-logr/logr"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	. "github.com/redhatinsights/xjoin-operator/controllers/index"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const xjoinindexvalidatorFinalizer = "finalizer.xjoin.indexvalidator.cloud.redhat.com"

type XJoinIndexValidatorReconciler struct {
	Client    client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Namespace string
	Test      bool
}

func NewXJoinIndexValidatorReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
	recorder record.EventRecorder,
	namespace string,
	isTest bool) *XJoinIndexValidatorReconciler {

	return &XJoinIndexValidatorReconciler{
		Client:    client,
		Log:       log,
		Scheme:    scheme,
		Recorder:  recorder,
		Namespace: namespace,
		Test:      isTest,
	}
}

func (r *XJoinIndexValidatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-indexvalidator-controller").
		For(&xjoin.XJoinIndexValidator{}).
		WithLogger(mgr.GetLogger()).
		WithOptions(controller.Options{
			Log: mgr.GetLogger(),
		}).
		Complete(r)
}

func (r *XJoinIndexValidatorReconciler) finalize(i XJoinIndexValidatorIteration) (result ctrl.Result, err error) {

	r.Log.Info("Starting finalizer")
	controllerutil.RemoveFinalizer(i.Instance, xjoinindexvalidatorFinalizer)

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = r.Client.Update(ctx, i.Instance)
	if err != nil {
		return
	}

	r.Log.Info("Successfully finalized")
	return reconcile.Result{}, nil
}

// +kubebuilder:rbac:groups=xjoin.cloud.redhat.com,resources=xjoinindexvalidator;xjoinindexvalidator/status;xjoinindexvalidator/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;pods,verbs=get;list;watch

func (r *XJoinIndexValidatorReconciler) Reconcile(ctx context.Context, request ctrl.Request) (result ctrl.Result, err error) {
	reqLogger := xjoinlogger.NewLogger("controller_xjoinindexvalidator", "IndexValidator", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling XJoinIndexValidator")

	instance, err := utils.FetchXJoinIndexValidator(r.Client, request.NamespacedName, ctx)
	if err != nil {
		if k8errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return result, nil
		}
		// Error reading the object - requeue the request.
		return
	}

	p := parameters.BuildIndexParameters()

	configManager, err := config.NewManager(config.ManagerOptions{
		Client:         r.Client,
		Parameters:     p,
		ConfigMapNames: []string{"xjoin"},
		SecretNames:    nil,
		Namespace:      instance.Namespace,
		Spec:           instance.Spec,
		Context:        ctx,
	})
	if err != nil {
		return
	}
	err = configManager.Parse()
	if err != nil {
		return
	}

	if p.Pause.Bool() == true {
		return
	}

	_ = XJoinIndexValidatorIteration{
		Instance:         instance,
		OriginalInstance: *instance.DeepCopy(),
		Client:           r.Client,
		Log:              reqLogger,
		Parameters:       *p,
	}

	return reconcile.Result{}, nil
}
