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
	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-index-controller").
		For(&xjoin.XJoinIndex{}).
		WithLogger(mgr.GetLogger()).
		WithOptions(controller.Options{
			Log:         mgr.GetLogger(),
			RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, 1*time.Minute),
		}).
		Watches(&source.Kind{Type: &xjoin.XJoinDataSource{}}, handler.EnqueueRequestsFromMapFunc(func(datasource client.Object) []reconcile.Request {
			//when a xjoindatasource spec is updated, reconcile each xjoinindex which consumes the xjoindatasource
			ctx, cancel := utils.DefaultContext()
			defer cancel()

			var requests []reconcile.Request

			indexes, err := utils.FetchXJoinIndexes(r.Client, ctx)
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

	if p.Pause.Bool() == true {
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

	//force refresh if datasource is updated
	forceRefresh := false
	for dataSourceName, dataSourceVersion := range instance.Status.DataSources {
		dataSourceNamespacedName := types.NamespacedName{
			Name:      dataSourceName,
			Namespace: instance.GetNamespace(),
		}
		dataSource, err := utils.FetchXJoinDataSource(i.Client, dataSourceNamespacedName, i.Context)
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

	instance.Status.SpecHash, err = utils.SpecHash(instance.Spec)
	if err != nil {
		return result, errors.Wrap(err, 0)
	}

	//TODO: actually validate
	if originalInstance.Status.RefreshingVersion != "" {
		instance.Status.ActiveVersionIsValid = true
		instance.Status.ActiveVersion = instance.Status.RefreshingVersion
	}

	if i.GetInstance().Status.DataSources == nil {
		i.GetInstance().Status.DataSources = map[string]string{}
	}

	return i.UpdateStatusAndRequeue()
}
