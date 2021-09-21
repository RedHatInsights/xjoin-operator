package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	. "github.com/redhatinsights/xjoin-operator/controllers/datasource"
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
	"strconv"
	"time"
)

const xjoindatasourceFinalizer = "finalizer.xjoin.datasource.cloud.redhat.com"

type XJoinDataSourceReconciler struct {
	Client    client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Namespace string
	Test      bool
}

func NewXJoinDataSourceReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
	recorder record.EventRecorder,
	namespace string,
	isTest bool) *XJoinDataSourceReconciler {

	return &XJoinDataSourceReconciler{
		Client:    client,
		Log:       log,
		Scheme:    scheme,
		Recorder:  recorder,
		Namespace: namespace,
		Test:      isTest,
	}
}

func (r *XJoinDataSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-datasource-controller").
		For(&xjoin.XJoinDataSource{}).
		WithLogger(mgr.GetLogger()).
		WithOptions(controller.Options{
			Log: mgr.GetLogger(),
		}).
		Complete(r)
}

func (r *XJoinDataSourceReconciler) finalize(i XJoinDataSourceInstance) (result ctrl.Result, err error) {

	r.Log.Info("Starting finalizer")
	err = i.DeleteDataSourcePipeline(i.Instance.Name, i.Instance.Status.ActiveVersion)
	if err != nil {
		return
	}
	err = i.DeleteDataSourcePipeline(i.Instance.Name, i.Instance.Status.RefreshingVersion)
	if err != nil {
		return
	}

	controllerutil.RemoveFinalizer(i.Instance, xjoindatasourceFinalizer)
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = r.Client.Update(ctx, i.Instance)
	if err != nil {
		return
	}

	r.Log.Info("Successfully finalized")
	return reconcile.Result{}, nil
}

// +kubebuilder:rbac:groups=xjoin.cloud.redhat.com,resources=xjoindatasources;xjoindatasources/status;xjoindatasources/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;pods,verbs=get;list;watch

func (r *XJoinDataSourceReconciler) Reconcile(ctx context.Context, request ctrl.Request) (result ctrl.Result, err error) {
	reqLogger := xjoinlogger.NewLogger("controller_xjoindatasource", "DataSource", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling XJoinDataSource")

	instance, err := utils.FetchXJoinDataSource(r.Client, request.NamespacedName, ctx)
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

	p := parameters.BuildDataSourceParameters()

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

	i := XJoinDataSourceInstance{
		Instance:         instance,
		OriginalInstance: *instance.DeepCopy(),
		Client:           r.Client,
		Log:              reqLogger,
		Parameters:       *p,
	}

	//[REMOVED]
	if instance.GetDeletionTimestamp() != nil {
		i.Log.Debug("STATE: REMOVED")
		return r.finalize(i)
	}

	//[NEW]
	if instance.Status.ActiveVersion == "" && instance.Status.RefreshingVersion == "" {
		refreshingVersion := version()
		instance.Status.RefreshingVersion = refreshingVersion
		instance.Status.RefreshingVersionIsValid = false

		i.Log.Debug("STATE: NEW")
		err = i.CreateDataSourcePipeline(instance.Name, refreshingVersion)
		if err != nil {
			return
		}
	}

	//[INITIAL SYNC]
	if instance.Status.ActiveVersion == "" &&
		instance.Status.RefreshingVersionIsValid == false &&
		instance.Status.RefreshingVersion != "" {

		//TODO: noop? double check datasourcepipeline exists?
		i.Log.Debug("STATE: INITIAL_SYNC")
	}

	//[VALID] ACTIVE IS VALID
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == true {

		//TODO: noop? double check datasourcepipeline exists?
		i.Log.Debug("STATE: VALID")
	}

	//[START REFRESHING] ACTIVE IS INVALID, NOT REFRESHING YET
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == false &&
		instance.Status.RefreshingVersion == "" {

		i.Log.Debug("STATE: START REFRESH")
		refreshingVersion := version()
		instance.Status.RefreshingVersion = refreshingVersion

		err = i.CreateDataSourcePipeline(instance.Name, refreshingVersion)
	}

	//[REFRESHING] ACTIVE IS INVALID, REFRESHING IS INVALID
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == false &&
		instance.Status.RefreshingVersion != "" &&
		instance.Status.RefreshingVersionIsValid == false {

		//TODO: noop? double check both datasourcepipelines exist?
		i.Log.Debug("STATE: REFRESHING")
	}

	//[REFRESH COMPLETE] ACTIVE IS INVALID, REFRESHING IS VALID
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == false &&
		instance.Status.RefreshingVersion != "" &&
		instance.Status.RefreshingVersionIsValid == true {

		i.Log.Debug("STATE: REFRESH COMPLETE")
		err = i.DeleteDataSourcePipeline(i.Instance.Name, i.Instance.Status.ActiveVersion)
		if err != nil {
			return
		}

		instance.Status.ActiveVersion = instance.Status.RefreshingVersion
		instance.Status.ActiveVersionIsValid = instance.Status.RefreshingVersionIsValid
	}

	return i.UpdateStatusAndRequeue()
}

func version() string {
	return fmt.Sprintf("%s", strconv.FormatInt(time.Now().UnixNano(), 10))
}
