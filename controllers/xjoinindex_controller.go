package controllers

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	. "github.com/redhatinsights/xjoin-operator/controllers/index"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
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
	logConstructor := func(r *reconcile.Request) logr.Logger {
		return mgr.GetLogger()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-index-controller").
		For(&xjoin.XJoinIndex{}).
		WithLogConstructor(logConstructor).
		WithOptions(controller.Options{
			LogConstructor: logConstructor,
			RateLimiter:    workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, 1*time.Minute),
		}).
		Watches(&source.Kind{Type: &xjoin.XJoinDataSource{}}, handler.EnqueueRequestsFromMapFunc(func(datasource client.Object) []reconcile.Request {
			//when a xjoindatasource spec is updated, reconcile each xjoinindex which consumes the xjoindatasource
			ctx, cancel := utils.DefaultContext()
			defer cancel()

			var requests []reconcile.Request

			indexes, err := k8sUtils.FetchXJoinIndexes(r.Client, ctx)
			if err != nil {
				r.Log.Error(err, "Failed to fetch XJoinIndexes")
				return requests
			}

			for _, xjoinIndex := range indexes.Items {
				if utils.ContainsString(xjoinIndex.GetDataSourceNames(), datasource.GetName()) {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Namespace: xjoinIndex.GetNamespace(),
							Name:      xjoinIndex.GetName(),
						},
					})
				}
			}

			r.Log.Info("XJoin DataSource changed. Reconciling related XJoinIndexes", "indexes", requests)
			return requests
		})).
		Complete(r)
}

// +kubebuilder:rbac:groups=xjoin.cloud.redhat.com,resources=xjoinindices;xjoinindices/status;xjoinindices/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;pods;deployments,verbs=get;list;watch

func (r *XJoinIndexReconciler) Reconcile(ctx context.Context, request ctrl.Request) (result ctrl.Result, err error) {
	reqLogger := xjoinlogger.NewLogger("controller_xjoinindex", "Index", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling XJoinIndex")

	instance, err := k8sUtils.FetchXJoinIndex(r.Client, request.NamespacedName, ctx)
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

	if p.Pause.Bool() {
		return
	}

	configManager, err := config.NewManager(config.ManagerOptions{
		Client:         r.Client,
		Parameters:     p,
		ConfigMapNames: []string{"xjoin-generic"},
		SecretNames:    []string{"xjoin-elasticsearch"},
		Namespace:      instance.Namespace,
		Spec:           instance.Spec,
		Context:        ctx,
	})
	if err != nil {
		return result, errors.Wrap(err, 0)
	}
	err = configManager.Parse()
	if err != nil {
		return result, errors.Wrap(err, 0)
	}

	originalInstance := instance.DeepCopy()
	i := XJoinIndexIteration{
		Parameters: *p,
		Iteration: common.Iteration{
			Context:          ctx,
			Instance:         instance,
			OriginalInstance: originalInstance,
			Client:           r.Client,
			Log:              reqLogger,
			Test:             r.Test,
		},
	}

	if err = i.AddFinalizer(i.GetFinalizerName()); err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	//check status of active and refreshing IndexPipelines, update instance.Status accordingly
	if instance.Status.ActiveVersion != "" {
		indexPipelineNamespacedName := types.NamespacedName{
			Name:      i.Instance.GetName() + "." + instance.Status.ActiveVersion,
			Namespace: i.Instance.GetNamespace(),
		}

		activeIndexPipeline, err := k8sUtils.FetchXJoinIndexPipeline(i.Client, indexPipelineNamespacedName, i.Context)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, 0)
		}

		instance.Status.ActiveVersionIsValid = activeIndexPipeline.Status.ValidationResponse.Result == "valid"
	}

	if instance.Status.RefreshingVersion != "" {
		indexPipelineNamespacedName := types.NamespacedName{
			Name:      i.Instance.GetName() + "." + instance.Status.RefreshingVersion,
			Namespace: i.Instance.GetNamespace(),
		}

		refreshingIndexPipeline, err := k8sUtils.FetchXJoinIndexPipeline(i.Client, indexPipelineNamespacedName, i.Context)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, 0)
		}

		instance.Status.RefreshingVersionIsValid = refreshingIndexPipeline.Status.ValidationResponse.Result == "valid"
	}

	//force refresh if datasource is updated
	forceRefresh := false
	for dataSourceName, dataSourceVersion := range instance.Status.DataSources {
		dataSourceNamespacedName := types.NamespacedName{
			Name:      dataSourceName,
			Namespace: instance.GetNamespace(),
		}
		dataSource, err := k8sUtils.FetchXJoinDataSource(i.Client, dataSourceNamespacedName, i.Context)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, 0)
		}
		if dataSource.ResourceVersion != dataSourceVersion {
			forceRefresh = true
		}
	}

	indexReconcileMethods := NewReconcileMethods(i, common.IndexGVK)
	reconciler := common.NewReconciler(indexReconcileMethods, instance, reqLogger)
	err = reconciler.Reconcile(forceRefresh)
	if err != nil {
		return result, errors.Wrap(err, 0)
	}

	if instance.GetDeletionTimestamp() != nil {
		//actual finalizer code is called via reconciler
		return reconcile.Result{}, nil
	}

	instance.Status.SpecHash, err = k8sUtils.SpecHash(instance.Spec)
	if err != nil {
		return result, errors.Wrap(err, 0)
	}

	if i.GetInstance().Status.DataSources == nil {
		i.GetInstance().Status.DataSources = map[string]string{}
	}

	return i.UpdateStatusAndRequeue(time.Second * 30)
}
