package controllers

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type XJoinIndexReconciler struct {
	Client    client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Namespace string
	Test      bool
}

func NewXJoinIndexReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
	recorder record.EventRecorder,
	namespace string,
	isTest bool) *XJoinIndexReconciler {

	return &XJoinIndexReconciler{
		Client:    client,
		Log:       log,
		Scheme:    scheme,
		Recorder:  recorder,
		Namespace: namespace,
		Test:      isTest,
	}
}

func (r *XJoinIndexReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-index-controller").
		For(&xjoin.XJoinIndex{}).
		WithLogger(mgr.GetLogger()).
		WithOptions(controller.Options{
			Log: mgr.GetLogger(),
		}).
		Complete(r)
}

// +kubebuilder:rbac:groups=xjoin.cloud.redhat.com,resources=xjoinindices;xjoinindices/status;xjoinindices/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;pods,verbs=get;list;watch

func (r *XJoinIndexReconciler) Reconcile(ctx context.Context, request ctrl.Request) (result ctrl.Result, err error) {
	reqLogger := xjoinlogger.NewLogger("controller_xjoinindex", "Index", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling XJoinIndex")

	instance, err := utils.FetchXJoinIndex(r.Client, request.NamespacedName, ctx)
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

	i := XJoinIndexIteration{
		Parameters: *p,
		Iteration: common.Iteration{
			Context:          ctx,
			Instance:         instance,
			OriginalInstance: instance.DeepCopy(),
			Client:           r.Client,
			Log:              reqLogger,
		},
	}

	if err = i.AddFinalizer(i.GetFinalizerName()); err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	indexReconcileMethods := NewReconcileMethods(i)
	reconciler := common.NewReconciler(indexReconcileMethods, instance, reqLogger)
	err = reconciler.Reconcile()
	if err != nil {
		return result, errors.Wrap(err, 0)
	}

	return i.UpdateStatusAndRequeue()
}
