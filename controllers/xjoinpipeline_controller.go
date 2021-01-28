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
type XJoinPipelineReconciler struct {
	Client   client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

const xjoinpipelineFinalizer = "finalizer.xjoin.cloud.redhat.com"

func (r *XJoinPipelineReconciler) setup(reqLogger xjoinlogger.Log, request ctrl.Request) (ReconcileIteration, error) {

	i := ReconcileIteration{}

	instance, err := utils.FetchXJoinPipeline(r.Client, request.NamespacedName)
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

	i = ReconcileIteration{
		Instance:         instance,
		OriginalInstance: instance.DeepCopy(),
		Scheme:           r.Scheme,
		Log:              reqLogger,
		Client:           r.Client,
		Recorder:         r.Recorder,
	}

	xjoinConfig, err := config.NewConfig(i.Instance, i.Client)
	if err != nil {
		return i, err
	}
	i.parameters = xjoinConfig.Parameters

	i.GetRequeueInterval = func(Instance *ReconcileIteration) int {
		return i.parameters.StandardInterval.Int()
	}

	es, err := elasticsearch.NewElasticSearch(
		i.parameters.ElasticSearchURL.String(),
		i.parameters.ElasticSearchUsername.String(),
		i.parameters.ElasticSearchPassword.String(),
		i.parameters.ResourceNamePrefix.String())

	if err != nil {
		return i, err
	}
	i.ESClient = es

	i.Kafka = kafka.Kafka{
		Namespace:     instance.Namespace,
		Client:        i.Client,
		Parameters:    i.parameters,
		ParametersMap: xjoinConfig.ParametersMap,
	}

	i.InventoryDb = database.NewDatabase(database.DBParams{
		Host:     i.parameters.HBIDBHost.String(),
		User:     i.parameters.HBIDBUser.String(),
		Name:     i.parameters.HBIDBName.String(),
		Port:     i.parameters.HBIDBPort.String(),
		Password: i.parameters.HBIDBPassword.String(),
	})

	if err = i.InventoryDb.Connect(); err != nil {
		return i, err
	}

	return i, nil
}

func (r *XJoinPipelineReconciler) Finalize(i ReconcileIteration) error {
	err := i.Kafka.DeleteConnectorsForPipelineVersion(i.Instance.Status.PipelineVersion)
	if err != nil {
		return err
	}

	err = i.InventoryDb.RemoveReplicationSlotsForPipelineVersion(i.Instance.Status.PipelineVersion)
	if err != nil {
		return err
	}

	err = i.Kafka.DeleteTopicByPipelineVersion(i.Instance.Status.PipelineVersion)
	if err != nil {
		return err
	}

	err = i.ESClient.DeleteIndex(i.Instance.Status.PipelineVersion)
	if err != nil {
		return err
	}
	return nil
}

// +kubebuilder:rbac:groups=xjoin.cloud.redhat.com,resources=xjoinpipelines;xjoinpipelines/status;xjoinpipelines/finalizers,verbs=*
// +kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkaconnectors;kafkaconnectors/finalizers,verbs=*
// +kubebuilder:rbac:groups="",resources=configmaps;secrets,verbs=get;list;watch

func (r *XJoinPipelineReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	reqLogger := xjoinlogger.NewLogger("controller_xjoinpipeline", "Pipeline", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling XJoinPipeline")

	i, err := r.setup(reqLogger, request)
	defer i.Close()

	if err != nil {
		i.error(err)
		return reconcile.Result{}, err
	}

	// Request object not found, could have been deleted after reconcile request.
	if i.Instance == nil {
		return reconcile.Result{}, nil
	}

	// remove any stale dependencies
	// if we're shutting down this removes all dependencies
	xjoinErrors := i.deleteStaleDependencies()

	for _, err := range xjoinErrors {
		i.error(err, "Error deleting stale dependency")
	}

	// STATE_REMOVED
	if i.Instance.GetState() == xjoin.STATE_REMOVED {
		if len(xjoinErrors) > 0 {
			return reconcile.Result{}, xjoinErrors[0]
		}

		err = i.Kafka.DeleteConnectorsForPipelineVersion(i.Instance.Status.PipelineVersion)
		if err != nil {
			i.error(err, "Error deleting connectors")
			return reconcile.Result{}, err
		}

		err = i.InventoryDb.RemoveReplicationSlotsForPipelineVersion(i.Instance.Status.PipelineVersion)
		if err != nil {
			i.error(err, "Error removing replication slots")
			return reconcile.Result{}, err
		}

		err = i.ESClient.DeleteIndex(i.Instance.Status.PipelineVersion)
		if err != nil {
			i.error(err, "Error removing ES indices")
			return reconcile.Result{}, err
		}

		err = i.Kafka.DeleteConnectorsForPipelineVersion(i.Instance.Status.PipelineVersion)
		if err != nil {
			i.error(err, "Error deleting connectors")
			return reconcile.Result{}, err
		}

		if err = i.removeFinalizer(); err != nil {
			i.error(err, "Error removing finalizer")
			return reconcile.Result{}, err
		}

		i.Log.Info("Successfully finalized XJoinPipeline")
		return reconcile.Result{}, nil
	}

	metrics.InitLabels(i.Instance)

	// STATE_NEW
	if i.Instance.GetState() == xjoin.STATE_NEW {
		if err := i.addFinalizer(); err != nil {
			i.error(err, "Error adding finalizer")
			return reconcile.Result{}, err
		}

		i.Instance.Status.XJoinConfigVersion = i.parameters.ConfigMapVersion.String()
		i.Instance.Status.ElasticSearchSecretVersion = i.parameters.ElasticSearchSecretVersion.String()
		i.Instance.Status.HBIDBSecretVersion = i.parameters.HBIDBSecretVersion.String()

		pipelineVersion := fmt.Sprintf("%s", strconv.FormatInt(time.Now().UnixNano(), 10))
		if err := i.Instance.TransitionToInitialSync(i.parameters.ResourceNamePrefix.String(), pipelineVersion); err != nil {
			i.error(err, "Error transitioning to Initial Sync")
			return reconcile.Result{}, err
		}
		i.probeStartingInitialSync()

		err := i.Kafka.CreateTopic(pipelineVersion)
		if err != nil {
			i.error(err, "Error creating Kafka topic")
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
		Watches(&source.Kind{Type: &v1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{

			ToRequests: handler.ToRequestsFunc(
				func(configMap handler.MapObject) []reconcile.Request {
					var requests []reconcile.Request

					if configMap.Meta.GetName() != "xjoin" {
						return requests
					}

					pipelines, err := utils.FetchXJoinPipelines(r.Client)
					if err != nil {
						r.Log.Error(err, "Failed to fetch XJoinPipelines")
						return requests
					}

					for _, pipeline := range pipelines.Items {
						requests = append(requests, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Namespace: configMap.Meta.GetNamespace(),
								Name:      pipeline.GetName(),
							},
						})
					}

					r.Log.Info("XJoin ConfigMap changed. Reconciling XJoinPipelines",
						"namespace", configMap.Meta.GetNamespace(), "pipelines", requests)
					return requests
				}),
		}).
		Watches(&source.Kind{Type: &v1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(secret handler.MapObject) []reconcile.Request {
					var requests []reconcile.Request

					secretName := secret.Meta.GetName()

					pipelines, err := utils.FetchXJoinPipelines(r.Client)
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
						"namespace", secret.Meta.GetNamespace(), "name", secret.Meta.GetName(), "pipelines", requests)
					return requests
				}),
		}).
		Complete(r)
}

func NewXJoinReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
	recorder record.EventRecorder) *XJoinPipelineReconciler {

	return &XJoinPipelineReconciler{
		Client:   client,
		Log:      log,
		Scheme:   scheme,
		Recorder: recorder,
	}
}
