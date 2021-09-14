/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	v1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strconv"
	"time"
)

// XJoinPipelineReconciler reconciles a XJoinPipeline object
const xjoinpipelineFinalizer = "finalizer.xjoin.cloud.redhat.com"

type XJoinPipelineReconciler struct {
	Client    client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Namespace string
	Test      bool
}

func (r *XJoinPipelineReconciler) setup(reqLogger xjoinlogger.Log, request ctrl.Request, ctx context.Context) (ReconcileIteration, error) {

	i := ReconcileIteration{}

	instance, err := utils.FetchXJoinPipeline(r.Client, request.NamespacedName, ctx)
	if err != nil {
		if k8errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return i, nil
		}
		// Error reading the object - requeue the request.
		return i, err
	}

	if instance.Spec.Pause == true {
		return i, nil
	}

	i = ReconcileIteration{
		Instance:         instance,
		OriginalInstance: instance.DeepCopy(),
		Scheme:           r.Scheme,
		Log:              reqLogger,
		Client:           r.Client,
		Recorder:         r.Recorder,
	}

	xjoinConfig, err := config.NewConfig(i.Instance, i.Client, ctx)
	if xjoinConfig != nil {
		i.parameters = xjoinConfig.Parameters
	}

	if err != nil {
		return i, err
	}

	i.GetRequeueInterval = func(Instance *ReconcileIteration) int {
		return i.parameters.StandardInterval.Int()
	}

	es, err := elasticsearch.NewElasticSearch(
		i.parameters.ElasticSearchURL.String(),
		i.parameters.ElasticSearchUsername.String(),
		i.parameters.ElasticSearchPassword.String(),
		i.parameters.ResourceNamePrefix.String(),
		i.parameters.ElasticSearchPipelineTemplate.String(),
		i.parameters.ElasticSearchIndexTemplate.String(),
		xjoinConfig.ParametersMap)

	if err != nil {
		return i, err
	}
	i.ESClient = es

	i.Kafka = kafka.Kafka{
		Namespace:     instance.Namespace,
		Client:        i.Client,
		Parameters:    i.parameters,
		ParametersMap: xjoinConfig.ParametersMap,
		Recorder:      i.Recorder,
		Test:          r.Test,
	}

	i.InventoryDb = database.NewDatabase(database.DBParams{
		Host:        i.parameters.HBIDBHost.String(),
		User:        i.parameters.HBIDBUser.String(),
		Name:        i.parameters.HBIDBName.String(),
		Port:        i.parameters.HBIDBPort.String(),
		Password:    i.parameters.HBIDBPassword.String(),
		SSLMode:     i.parameters.HBIDBSSLMode.String(),
		SSLRootCert: i.parameters.HBIDBSSLRootCert.String(),
	})

	if err = i.InventoryDb.Connect(); err != nil {
		return i, err
	}

	return i, nil
}

// +kubebuilder:rbac:groups=xjoin.cloud.redhat.com,resources=xjoinpipelines;xjoinpipelines/status;xjoinpipelines/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkaconnectors;kafkaconnectors/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkatopics;kafkatopics/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkaconnects;kafkas,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps;secrets;pods,verbs=get;list;watch

func (r *XJoinPipelineReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var setupErrors []error

	reqLogger := xjoinlogger.NewLogger("controller_xjoinpipeline", "Pipeline", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling XJoinPipeline")

	i, err := r.setup(reqLogger, request, ctx)
	defer i.Close()

	if err != nil {
		i.error(err)
		setupErrors = append(setupErrors, err)
	}

	// Request object not found, could have been deleted after reconcile request or
	if i.Instance == nil {
		return reconcile.Result{}, nil
	}

	//pause this pipeline. Reconcile loop is skipped until Pause is set to false or nil
	if i.Instance.Spec.Pause == true {
		return reconcile.Result{}, nil
	}

	// remove any stale dependencies
	// if we're shutting down this removes all dependencies
	if len(setupErrors) < 1 {
		setupErrors = append(setupErrors, i.deleteStaleDependencies()...)
	}

	for _, err = range setupErrors {
		i.error(err, "Error deleting stale dependency")
	}

	// STATE_REMOVED
	if i.Instance.GetState() == xjoin.STATE_REMOVED {
		if len(setupErrors) > 0 && !i.parameters.Ephemeral.Bool() {
			return reconcile.Result{}, setupErrors[0]
		} else if len(setupErrors) > 0 && i.parameters.Ephemeral.Bool() {
			//remove finalizer without deleting deps in ephemeral env when an error occurred loading configuration params
			err = i.removeFinalizer()
			if err != nil {
				i.error(err, "Error removing finalizer")
				return reconcile.Result{}, err
			} else {
				return reconcile.Result{}, nil
			}
		}

		err = i.DeleteResourceForPipeline(i.Instance.Status.PipelineVersion)
		err = i.DeleteResourceForPipeline(i.Instance.Status.ActivePipelineVersion)

		//allow this to fail in ephemeral envs because
		//DeleteResourceForPipeline could fail due to Kafka/KafkaConnect already being deleted
		if err != nil && !i.parameters.Ephemeral.Bool() {
			return reconcile.Result{}, err
		}

		if err = i.removeFinalizer(); err != nil {
			i.error(err, "Error removing finalizer")
			return reconcile.Result{}, err
		}

		i.Log.Info("Successfully finalized XJoinPipeline")
		return reconcile.Result{}, nil
	}

	if len(setupErrors) > 0 {
		return reconcile.Result{}, setupErrors[0]
	}

	metrics.InitLabels()

	// STATE_NEW
	if i.Instance.GetState() == xjoin.STATE_NEW {
		if err = i.addFinalizer(); err != nil {
			i.error(err, "Error adding finalizer")
			return reconcile.Result{}, err
		}

		i.Instance.Status.XJoinConfigVersion = i.parameters.ConfigMapVersion.String()
		i.Instance.Status.ElasticSearchSecretVersion = i.parameters.ElasticSearchSecretVersion.String()
		i.Instance.Status.HBIDBSecretVersion = i.parameters.HBIDBSecretVersion.String()

		pipelineVersion := fmt.Sprintf("%s", strconv.FormatInt(time.Now().UnixNano(), 10))
		if err = i.Instance.TransitionToInitialSync(i.parameters.ResourceNamePrefix.String(), pipelineVersion); err != nil {
			i.error(err, "Error transitioning to Initial Sync")
			return reconcile.Result{}, err
		}
		i.probeStartingInitialSync()

		_, err = i.Kafka.CreateTopic(pipelineVersion, false)
		if err != nil {
			i.error(err, "Error creating Kafka topic")
			return reconcile.Result{}, err
		}

		err = i.ESClient.CreateESPipeline(pipelineVersion)
		if err != nil {
			i.error(err, "Error creating ElasticSearch pipeline")
			return reconcile.Result{}, err
		}

		err = i.ESClient.CreateIndex(pipelineVersion)
		if err != nil {
			i.error(err, "Error creating ElasticSearch index")
			return reconcile.Result{}, err
		}

		_, err = i.Kafka.CreateDebeziumConnector(pipelineVersion, false)
		if err != nil {
			i.error(err, "Error creating debezium connector")
			return reconcile.Result{}, err
		}

		_, err = i.Kafka.CreateESConnector(pipelineVersion, false)
		if err != nil {
			i.error(err, "Error creating ES connector")
			return reconcile.Result{}, err
		}

		i.Log.Info("Transitioning to InitialSync")
		return i.updateStatusAndRequeue()
	}

	// STATE_VALID
	if i.Instance.GetState() == xjoin.STATE_VALID {
		i.setActiveResources()
		if updated, err := i.recreateAliasIfNeeded(); err != nil {
			i.error(err, "Error updating hosts view")
			return reconcile.Result{}, err
		} else if updated {
			i.eventNormal(
				"ValidationSucceeded",
				"Pipeline became valid. xjoin.inventory.hosts alias now points to %s",
				i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion))
		}
		return i.updateStatusAndRequeue()
	}

	// invalid pipeline - either STATE_INITIAL_SYNC or STATE_INVALID
	if i.Instance.GetValid() == metav1.ConditionFalse {
		if i.Instance.Status.ValidationFailedCount >= i.getValidationAttemptsThreshold() {

			// This pipeline never became valid.
			if i.Instance.GetState() == xjoin.STATE_INITIAL_SYNC {
				if err = i.updateAliasIfHealthier(); err != nil {
					// if this fails continue and do refresh, keeping the old index active
					i.Log.Error(err, "Failed to evaluate which table is healthier")
				}
			}

			i.Instance.TransitionToNew()
			i.probePipelineDidNotBecomeValid()
			return i.updateStatusAndRequeue()
		}
	}

	return i.updateStatusAndRequeue()
}

func (r *XJoinPipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-controller").
		For(&xjoin.XJoinPipeline{}).
		Owns(kafka.EmptyConnector()).
		// trigger Reconcile if ConfigMap changes
		Watches(&source.Kind{Type: &v1.ConfigMap{}}, handler.EnqueueRequestsFromMapFunc(func(configMap client.Object) []reconcile.Request {
			ctx, cancel := utils.DefaultContext()
			defer cancel()

			var requests []reconcile.Request

			if configMap.GetNamespace() != r.Namespace || configMap.GetName() != "xjoin" {
				return requests
			}

			pipelines, err := utils.FetchXJoinPipelines(r.Client, ctx)
			if err != nil {
				r.Log.Error(err, "Failed to fetch XJoinPipelines")
				return requests
			}

			for _, pipeline := range pipelines.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: configMap.GetNamespace(),
						Name:      pipeline.GetName(),
					},
				})
			}

			r.Log.Info("XJoin ConfigMap changed. Reconciling XJoinPipelines",
				"namespace", configMap.GetNamespace(), "pipelines", requests)
			return requests
		})).
		Watches(&source.Kind{Type: &v1.Secret{}}, handler.EnqueueRequestsFromMapFunc(func(secret client.Object) []reconcile.Request {
			ctx, cancel := utils.DefaultContext()
			defer cancel()

			var requests []reconcile.Request

			if secret.GetNamespace() != r.Namespace {
				return requests
			}

			secretName := secret.GetName()

			pipelines, err := utils.FetchXJoinPipelines(r.Client, ctx)
			if err != nil {
				r.Log.Error(err, "Failed to fetch XJoinPipelines")
				return requests
			}

			for _, pipeline := range pipelines.Items {
				if pipeline.Status.HBIDBSecretName == secretName || pipeline.Status.ElasticSearchSecretName == secretName {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Namespace: pipeline.GetNamespace(),
							Name:      pipeline.GetName(),
						},
					})
				}
			}

			r.Log.Info("XJoin secret changed. Reconciling XJoinPipelines",
				"namespace", secret.GetNamespace(), "name", secret.GetName(), "pipelines", requests)
			return requests
		})).
		Complete(r)
}

func NewXJoinReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
	recorder record.EventRecorder,
	namespace string,
	isTest bool) *XJoinPipelineReconciler {

	return &XJoinPipelineReconciler{
		Client:    client,
		Log:       log,
		Scheme:    scheme,
		Recorder:  recorder,
		Namespace: namespace,
		Test:      isTest,
	}
}
