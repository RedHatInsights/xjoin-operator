package controllers

import (
	"context"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const xjoinindexFinalizer = "finalizer.xjoin.index.cloud.redhat.com"

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

func (r *XJoinIndexReconciler) finalize(i XJoinIndexIteration) (result ctrl.Result, err error) {

	r.Log.Info("Starting finalizer")
	controllerutil.RemoveFinalizer(i.Iteration.Instance, xjoinindexFinalizer)

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = r.Client.Update(ctx, i.Iteration.Instance)
	if err != nil {
		return
	}

	r.Log.Info("Successfully finalized")
	return reconcile.Result{}, nil
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

	//[REMOVED]
	if instance.GetDeletionTimestamp() != nil {
		i.Log.Debug("STATE: REMOVED")
		return r.finalize(i)
	}

	//[NEW]
	if instance.Status.ActiveVersion == "" && instance.Status.RefreshingVersion == "" {
		refreshingVersion := i.Version()
		instance.Status.RefreshingVersion = refreshingVersion
		instance.Status.RefreshingVersionIsValid = false

		i.Log.Debug("STATE: NEW")
		err = i.CreateIndexPipeline(instance.Name, refreshingVersion)
		if err != nil {
			return
		}
		err = i.CreateIndexValidator(instance.Name, refreshingVersion)
		if err != nil {
			return
		}
	}

	//[INITIAL SYNC]
	if instance.Status.ActiveVersion == "" &&
		instance.Status.RefreshingVersionIsValid == false &&
		instance.Status.RefreshingVersion != "" {

		//TODO: noop? double check indexpipeline exists?
		i.Log.Debug("STATE: INITIAL_SYNC")
	}

	//[VALID] ACTIVE IS VALID
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == true {

		//TODO: noop? double check indexpipeline exists?
		i.Log.Debug("STATE: VALID")
	}

	//[START REFRESHING] ACTIVE IS INVALID, NOT REFRESHING YET
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == false &&
		instance.Status.RefreshingVersion == "" {

		i.Log.Debug("STATE: START REFRESH")
		refreshingVersion := i.Version()
		instance.Status.RefreshingVersion = refreshingVersion

		err = i.CreateIndexPipeline(instance.Name, refreshingVersion)
		if err != nil {
			return
		}
		err = i.CreateIndexValidator(instance.Name, refreshingVersion)
		if err != nil {
			return
		}
	}

	//[REFRESHING] ACTIVE IS INVALID, REFRESHING IS INVALID
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == false &&
		instance.Status.RefreshingVersion != "" &&
		instance.Status.RefreshingVersionIsValid == false {

		//TODO: noop? double check both indexpipelines exist?
		i.Log.Debug("STATE: REFRESHING")
	}

	//[REFRESH COMPLETE] ACTIVE IS INVALID, REFRESHING IS VALID
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == false &&
		instance.Status.RefreshingVersion != "" &&
		instance.Status.RefreshingVersionIsValid == true {

		i.Log.Debug("STATE: REFRESH COMPLETE")
		err = i.DeleteIndexPipeline(i.Iteration.Instance.GetName(), i.GetInstance().Status.ActiveVersion)
		if err != nil {
			return
		}
		err = i.DeleteIndexValidator(i.Iteration.Instance.GetName(), i.GetInstance().Status.ActiveVersion)
		if err != nil {
			return
		}

		instance.Status.ActiveVersion = instance.Status.RefreshingVersion
		instance.Status.ActiveVersionIsValid = instance.Status.RefreshingVersionIsValid
	}

	return i.UpdateStatusAndRequeue()
}
