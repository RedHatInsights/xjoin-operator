package controllers

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	. "github.com/redhatinsights/xjoin-operator/controllers/index"
	"github.com/redhatinsights/xjoin-operator/controllers/k8s"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

const xjoinindexValidatorFinalizer = "finalizer.xjoin.indexvalidator.cloud.redhat.com"

type XJoinIndexValidatorReconciler struct {
	Client       client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
	Namespace    string
	Test         bool
	ClientSet    kubernetes.Interface
	PodLogReader k8s.LogReader
}

func NewXJoinIndexValidatorReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	clientset kubernetes.Interface,
	log logr.Logger,
	recorder record.EventRecorder,
	namespace string,
	isTest bool,
	podLogReader k8s.LogReader) *XJoinIndexValidatorReconciler {

	return &XJoinIndexValidatorReconciler{
		Client:       client,
		Log:          log,
		Scheme:       scheme,
		Recorder:     recorder,
		Namespace:    namespace,
		Test:         isTest,
		ClientSet:    clientset,
		PodLogReader: podLogReader,
	}
}

func (r *XJoinIndexValidatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	logConstructor := func(r *reconcile.Request) logr.Logger {
		return mgr.GetLogger()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-indexvalidator-controller").
		For(&xjoin.XJoinIndexValidator{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithLogConstructor(logConstructor).
		WithOptions(controller.Options{
			LogConstructor: logConstructor,
			RateLimiter:    workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, 1*time.Minute),
		}).
		Complete(r)
}

// +kubebuilder:rbac:groups=xjoin.cloud.redhat.com,resources=xjoinindexvalidators;xjoinindexvalidators/status;xjoinindexvalidators/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;pods,verbs=get;list;watch

func (r *XJoinIndexValidatorReconciler) Reconcile(ctx context.Context, request ctrl.Request) (result ctrl.Result, err error) {
	reqLogger := xjoinlogger.NewLogger("controller_xjoinindexvalidator", "IndexValidator", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling XJoinIndexValidator")

	instance, err := k8sUtils.FetchXJoinIndexValidator(r.Client, request.NamespacedName, ctx)
	if err != nil {
		if k8errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			if r.Test {
				reqLogger.Error(
					err, "Unable to find XJoinIndexValidator", "name", request.Name)
			}
			return result, nil
		}
		// Error reading the object - requeue the request.
		return
	}

	p := parameters.BuildIndexParameters()

	var elasticsearchKubernetesSecretName string
	if instance.Spec.Ephemeral {
		elasticsearchKubernetesSecretName = "xjoin-elasticsearch-es-elastic-user"
	} else {
		elasticsearchKubernetesSecretName = "xjoin-elasticsearch"
	}

	configManager, err := config.NewManager(config.ManagerOptions{
		Client:         r.Client,
		Parameters:     p,
		ConfigMapNames: []string{"xjoin-generic"},
		SecretNames: []config.SecretNames{{
			KubernetesName: elasticsearchKubernetesSecretName,
			ManagerName:    parameters.ElasticsearchSecret,
		}},
		ResourceNamespace: instance.Namespace,
		OperatorNamespace: r.Namespace,
		Spec:              instance.Spec,
		Context:           ctx,
		Ephemeral:         instance.Spec.Ephemeral,
		Log:               reqLogger,
	})
	if err != nil {
		return
	}
	err = configManager.Parse()
	if err != nil {
		return
	}

	if p.Pause.Bool() {
		return
	}

	i := XJoinIndexValidatorIteration{
		Parameters: *p,
		Iteration: common.Iteration{
			Context:          ctx,
			Instance:         instance,
			OriginalInstance: instance.DeepCopy(),
			Client:           r.Client,
			Log:              reqLogger,
		},
		ClientSet:              r.ClientSet,
		ElasticsearchIndexName: instance.Spec.IndexName,
		PodLogReader:           r.PodLogReader,
	}

	if err = i.AddFinalizer(xjoinindexValidatorFinalizer); err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	if instance.GetDeletionTimestamp() != nil {
		err = i.Finalize()
		if err != nil {
			return result, errors.Wrap(err, 0)
		}
	}

	phase, err := i.ReconcileValidationPod()
	if err != nil {
		return result, errors.Wrap(err, 0)
	}
	instance.Status.ValidationPodPhase = phase
	if phase == ValidatorPodSuccess {
		return i.UpdateStatusAndRequeue(time.Second * time.Duration(p.ValidationInterval.Int()))
	} else if phase == ValidatorPodFailed {
		return i.UpdateStatusAndRequeue(time.Second * time.Duration(0))
	} else {
		return i.UpdateStatusAndRequeue(time.Second * time.Duration(p.ValidationPodStatusInterval.Int()))
	}
}
