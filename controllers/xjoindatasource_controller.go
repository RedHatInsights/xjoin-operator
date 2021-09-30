package controllers

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/components"
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

func (r *XJoinDataSourceReconciler) finalize(i XJoinDataSourceIteration) (result ctrl.Result, err error) {

	r.Log.Info("Starting finalizer")
	if i.GetInstance().Status.ActiveVersion != "" {
		err = i.DeleteDataSourcePipeline(i.GetInstance().Name, i.GetInstance().Status.ActiveVersion)
		if err != nil {
			return result, errors.Wrap(err, 0)
		}
	}
	if i.GetInstance().Status.RefreshingVersion != "" {
		err = i.DeleteDataSourcePipeline(i.GetInstance().Name, i.GetInstance().Status.RefreshingVersion)
		if err != nil {
			return result, errors.Wrap(err, 0)
		}
	}

	controllerutil.RemoveFinalizer(i.Instance, xjoindatasourceFinalizer)
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = r.Client.Update(ctx, i.Instance)
	if err != nil {
		return result, errors.Wrap(err, 0)
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

	i := XJoinDataSourceIteration{
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
		i.Log.Debug("STATE: NEW")

		if err = i.AddFinalizer(xjoindatasourceFinalizer); err != nil {
			return reconcile.Result{}, errors.Wrap(err, 0)
		}

		refreshingVersion := i.Version()
		instance.Status.RefreshingVersion = refreshingVersion
		instance.Status.RefreshingVersionIsValid = false

		err = i.CreateDataSourcePipeline(instance.Name, refreshingVersion)
		if err != nil {
			return
		}
	}

	//[INITIAL SYNC]
	if instance.Status.ActiveVersion == "" &&
		instance.Status.RefreshingVersionIsValid == false &&
		instance.Status.RefreshingVersion != "" {

		i.Log.Debug("STATE: INITIAL_SYNC")

		err = i.ReconcilePipelines()
		if err != nil {
			return result, errors.Wrap(err, 0)
		}
	}

	//[VALID] ACTIVE IS VALID
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == true {

		i.Log.Debug("STATE: VALID")

		err = i.ReconcilePipelines()
		if err != nil {
			return result, errors.Wrap(err, 0)
		}
	}

	//[START REFRESHING] ACTIVE IS INVALID, NOT REFRESHING YET
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == false &&
		instance.Status.RefreshingVersion == "" {

		i.Log.Debug("STATE: START REFRESH")
		refreshingVersion := i.Version()
		instance.Status.RefreshingVersion = refreshingVersion

		err = i.CreateDataSourcePipeline(instance.Name, refreshingVersion)
		if err != nil {
			return result, errors.Wrap(err, 0)
		}
	}

	//[REFRESHING] ACTIVE IS INVALID, REFRESHING IS INVALID
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == false &&
		instance.Status.RefreshingVersion != "" &&
		instance.Status.RefreshingVersionIsValid == false {

		i.Log.Debug("STATE: REFRESHING")

		err = i.ReconcilePipelines()
		if err != nil {
			return result, errors.Wrap(err, 0)
		}
	}

	//[REFRESH COMPLETE] ACTIVE IS INVALID, REFRESHING IS VALID
	if instance.Status.ActiveVersion != "" &&
		instance.Status.ActiveVersionIsValid == false &&
		instance.Status.RefreshingVersion != "" &&
		instance.Status.RefreshingVersionIsValid == true {

		i.Log.Debug("STATE: REFRESH COMPLETE")
		err = i.DeleteDataSourcePipeline(i.GetInstance().Name, i.GetInstance().Status.ActiveVersion)
		if err != nil {
			return result, errors.Wrap(err, 0)
		}

		instance.Status.ActiveVersion = instance.Status.RefreshingVersion
		instance.Status.ActiveVersionIsValid = instance.Status.RefreshingVersionIsValid
		instance.Status.RefreshingVersion = ""
		instance.Status.RefreshingVersionIsValid = false
	}

	//Scrub orphaned resources
	var validVersions []string
	if instance.Status.ActiveVersion != "" {
		validVersions = append(validVersions, instance.Status.ActiveVersion)
	}
	if instance.Status.RefreshingVersion != "" {
		validVersions = append(validVersions, instance.Status.RefreshingVersion)
	}

	custodian := components.NewCustodian(i.GetInstance().Name, validVersions)
	custodian.AddComponent(components.NewAvroSchema(""))
	custodian.AddComponent(components.NewKafkaTopic())
	custodian.AddComponent(components.NewDebeziumConnector())
	err = custodian.Scrub()
	if err != nil {
		return result, errors.Wrap(err, 0)
	}

	return i.UpdateStatusAndRequeue()
}
