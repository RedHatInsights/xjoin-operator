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
	. "github.com/redhatinsights/xjoin-operator/controllers/pipeline"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	v1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"strconv"
	"time"
)

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
		i.Parameters = xjoinConfig.Parameters
	}

	if err != nil {
		return i, err
	}

	i.GetRequeueInterval = func(Instance *ReconcileIteration) int {
		return i.Parameters.StandardInterval.Int()
	}

	es, err := elasticsearch.NewElasticSearch(
		i.Parameters.ElasticSearchURL.String(),
		i.Parameters.ElasticSearchUsername.String(),
		i.Parameters.ElasticSearchPassword.String(),
		i.Parameters.ResourceNamePrefix.String(),
		i.Parameters.ElasticSearchPipelineTemplate.String(),
		i.Parameters.ElasticSearchIndexTemplate.String(),
		xjoinConfig.ParametersMap)

	if err != nil {
		return i, err
	}
	i.ESClient = es

	i.Kafka = kafka.Kafka{
		Namespace:     instance.Namespace,
		Client:        i.Client,
		Parameters:    i.Parameters,
		ParametersMap: xjoinConfig.ParametersMap,
		Recorder:      i.Recorder,
		Test:          r.Test,
		GenericKafka: kafka.GenericKafka{
			Context:          ctx,
			Client:           i.Client,
			ConnectNamespace: i.Parameters.ConnectClusterNamespace.String(),
			ConnectCluster:   i.Parameters.ConnectCluster.String(),
			KafkaNamespace:   i.Parameters.KafkaClusterNamespace.String(),
			KafkaCluster:     i.Parameters.KafkaCluster.String(),
		},
	}

	topicParameters := kafka.TopicParameters{
		Replicas:           i.Parameters.KafkaTopicReplicas.Int(),
		Partitions:         i.Parameters.KafkaTopicPartitions.Int(),
		CleanupPolicy:      i.Parameters.KafkaTopicCleanupPolicy.String(),
		MinCompactionLagMS: i.Parameters.KafkaTopicMinCompactionLagMS.String(),
		RetentionBytes:     i.Parameters.KafkaTopicRetentionBytes.String(),
		RetentionMS:        i.Parameters.KafkaTopicRetentionMS.String(),
		MessageBytes:       i.Parameters.KafkaTopicMessageBytes.String(),
		CreationTimeout:    i.Parameters.KafkaTopicCreationTimeout.Int(),
	}

	if i.Parameters.ManagedKafka.Bool() == true {
		r.Log.Info("Loading Managed Kafka secret")
		managedKafkaSecret := &v1.Secret{}
		namespacedName := types.NamespacedName{
			Name:      i.Parameters.ManagedKafkaSecretName.String(),
			Namespace: i.Parameters.ManagedKafkaSecretNamespace.String(),
		}

		err = r.Client.Get(ctx, namespacedName, managedKafkaSecret)
		if err != nil {
			return i, err
		}

		i.KafkaTopics = kafka.NewManagedTopics(kafka.ManagedTopicsOptions{
			ResourceNamePrefix: i.Parameters.ResourceNamePrefix.String(),
			ClientId:           string(managedKafkaSecret.Data["client.id"]),
			ClientSecret:       string(managedKafkaSecret.Data["client.secret"]),
			Hostname:           string(managedKafkaSecret.Data["hostname"]),
			AdminURL:           string(managedKafkaSecret.Data["admin.url"]),
			TokenURL:           string(managedKafkaSecret.Data["token.url"]),
			TopicParameters:    topicParameters,
		})
	} else {
		r.Log.Info("Loading Strimzi parameters")
		i.KafkaTopics = &kafka.StrimziTopics{
			TopicParameters:       topicParameters,
			KafkaClusterNamespace: i.Parameters.KafkaClusterNamespace.String(),
			KafkaCluster:          i.Parameters.KafkaCluster.String(),
			Client:                i.Client,
			Test:                  r.Test,
			Context:               ctx,
			ResourceNamePrefix:    i.Parameters.ResourceNamePrefix.String(),
		}
	}

	i.KafkaConnectors = &kafka.StrimziConnectors{
		Kafka:  i.Kafka,
		Topics: i.KafkaTopics,
	}

	i.InventoryDb = database.NewDatabase(database.DBParams{
		Host:        i.Parameters.HBIDBHost.String(),
		User:        i.Parameters.HBIDBUser.String(),
		Name:        i.Parameters.HBIDBName.String(),
		Port:        i.Parameters.HBIDBPort.String(),
		Password:    i.Parameters.HBIDBPassword.String(),
		SSLMode:     i.Parameters.HBIDBSSLMode.String(),
		SSLRootCert: i.Parameters.HBIDBSSLRootCert.String(),
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
// +kubebuilder:rbac:groups=cloud.redhat.com,resources=clowdenvironments,verbs=get;list;watch

func (r *XJoinPipelineReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	var setupErrors []error

	reqLogger := xjoinlogger.NewLogger("controller_xjoinpipeline", "Pipeline", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling XJoinPipeline")

	i, err := r.setup(reqLogger, request, ctx)
	defer i.Close()

	if err != nil {
		i.Error(err)
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
		setupErrors = append(setupErrors, i.DeleteStaleDependencies()...)
	}

	for _, err = range setupErrors {
		i.Error(err, "Error deleting stale dependency")
	}

	// STATE_REMOVED
	if i.Instance.GetState() == xjoin.STATE_REMOVED {
		if len(setupErrors) > 0 && !i.Parameters.Ephemeral.Bool() {
			return reconcile.Result{}, setupErrors[0]
		} else if len(setupErrors) > 0 && i.Parameters.Ephemeral.Bool() {
			//remove finalizer without deleting deps in ephemeral env when an error occurred loading configuration params
			err = i.RemoveFinalizer()
			if err != nil {
				i.Error(err, "Error removing finalizer")
				return reconcile.Result{}, err
			} else {
				return reconcile.Result{}, nil
			}
		}

		err = i.DeleteResourceForPipeline(i.Instance.Status.PipelineVersion)
		err = i.DeleteResourceForPipeline(i.Instance.Status.ActivePipelineVersion)

		//allow this to fail in ephemeral envs because
		//DeleteResourceForPipeline could fail due to Kafka/KafkaConnect already being deleted
		if err != nil && !i.Parameters.Ephemeral.Bool() {
			return reconcile.Result{}, err
		}

		if err = i.RemoveFinalizer(); err != nil {
			i.Error(err, "Error removing finalizer")
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
		if err = i.AddFinalizer(); err != nil {
			i.Error(err, "Error adding finalizer")
			return reconcile.Result{}, err
		}

		i.Instance.Status.XJoinConfigVersion = i.Parameters.ConfigMapVersion.String()
		i.Instance.Status.ElasticSearchSecretVersion = i.Parameters.ElasticSearchSecretVersion.String()
		i.Instance.Status.HBIDBSecretVersion = i.Parameters.HBIDBSecretVersion.String()

		pipelineVersion := fmt.Sprintf("%s", strconv.FormatInt(time.Now().UnixNano(), 10))
		if err = i.Instance.TransitionToInitialSync(i.Parameters.ResourceNamePrefix.String(), pipelineVersion); err != nil {
			i.Error(err, "Error transitioning to Initial Sync")
			return reconcile.Result{}, err
		}
		i.ProbeStartingInitialSync()

		err = i.KafkaTopics.CreateTopic(pipelineVersion, false)
		if err != nil {
			i.Error(err, "Error creating Kafka topic")
			return reconcile.Result{}, err
		}

		err = i.ESClient.CreateESPipeline(pipelineVersion)
		if err != nil {
			i.Error(err, "Error creating ElasticSearch pipeline")
			return reconcile.Result{}, err
		}

		err = i.ESClient.CreateIndex(pipelineVersion)
		if err != nil {
			i.Error(err, "Error creating ElasticSearch index")
			return reconcile.Result{}, err
		}

		_, err = i.KafkaConnectors.CreateDebeziumConnector(pipelineVersion, false)
		if err != nil {
			i.Error(err, "Error creating debezium connector")
			return reconcile.Result{}, err
		}

		_, err = i.KafkaConnectors.CreateESConnector(pipelineVersion, false)
		if err != nil {
			i.Error(err, "Error creating ES connector")
			return reconcile.Result{}, err
		}

		i.Log.Info("Transitioning to InitialSync")
		return i.UpdateStatusAndRequeue()
	}

	// STATE_VALID
	if i.Instance.GetState() == xjoin.STATE_VALID {
		i.SetActiveResources()
		if updated, err := i.RecreateAliasIfNeeded(); err != nil {
			i.Error(err, "Error updating hosts view")
			return reconcile.Result{}, err
		} else if updated {
			i.EventNormal(
				"ValidationSucceeded",
				"Pipeline became valid. xjoin.inventory.hosts alias now points to %s",
				i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion))
		}
		return i.UpdateStatusAndRequeue()
	}

	// invalid pipeline - either STATE_INITIAL_SYNC or STATE_INVALID
	if i.Instance.GetValid() == metav1.ConditionFalse {
		if i.Instance.Status.ValidationFailedCount >= i.GetValidationAttemptsThreshold() {

			// This pipeline never became valid.
			if i.Instance.GetState() == xjoin.STATE_INITIAL_SYNC {
				if err = i.UpdateAliasIfHealthier(); err != nil {
					// if this fails continue and do refresh, keeping the old index active
					i.Log.Error(err, "Failed to evaluate which table is healthier")
				}
			}

			i.Instance.TransitionToNew()
			i.ProbePipelineDidNotBecomeValid()
			return i.UpdateStatusAndRequeue()
		}
	}

	return i.UpdateStatusAndRequeue()
}

func (r *XJoinPipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-controller").
		For(&xjoin.XJoinPipeline{}).
		Owns(kafka.EmptyConnector()).
		WithLogger(mgr.GetLogger()).
		WithOptions(controller.Options{
			Log:         mgr.GetLogger(),
			RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, 1*time.Minute),
		}).
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
