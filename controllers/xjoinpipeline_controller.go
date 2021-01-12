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
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
		i.parameters.ElasticSearchPassword.String())

	if err != nil {
		return i, err
	}
	i.ESClient = es

	i.Kafka = kafka.Kafka{
		Namespace:      instance.Namespace,
		ConnectCluster: i.parameters.ConnectCluster.String(),
		KafkaCluster:   i.parameters.KafkaCluster.String(),
		Owner:          instance,
		OwnerScheme:    i.Scheme,
		Client:         i.Client,
	}

	return i, nil
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

		pipelineVersion := fmt.Sprintf("%s", strconv.FormatInt(time.Now().UnixNano(), 10))
		if err := i.Instance.TransitionToInitialSync(pipelineVersion); err != nil {
			i.error(err, "Error transitioning to Initial Sync")
			return reconcile.Result{}, err
		}
		i.probeStartingInitialSync()

		err := i.Kafka.CreateTopic(pipelineVersion)
		if err != nil {
			i.error(err, "Error creating Kafka topic")
			return reconcile.Result{}, err
		}

		err = i.ESClient.CreateIndex(i.parameters.ResourceNamePrefix.String(), pipelineVersion)
		if err != nil {
			i.error(err, "Error creating ElasticSearch index")
			return reconcile.Result{}, err
		}

		_, err = i.Kafka.CreateDebeziumConnector(i.parameters, pipelineVersion, false)
		if err != nil {
			i.error(err, "Error creating debezium connector")
			return reconcile.Result{}, err
		}
		_, err = i.Kafka.CreateESConnector(i.parameters, pipelineVersion, false)
		if err != nil {
			i.error(err, "Error creating ES connector")
			return reconcile.Result{}, err
		}

		i.Log.Info("Transitioning to InitialSync")
		return i.updateStatusAndRequeue()
	}

	// STATE_VALID
	if i.Instance.GetState() == xjoin.STATE_VALID {
		if updated, err := i.recreateAliasIfNeeded(); err != nil {
			i.error(err, "Error updating hosts view")
			return reconcile.Result{}, err
		} else if updated {
			i.eventNormal(
				"ValidationSucceeded",
				"Pipeline became valid. xjoin.inventory.hosts alias now points to %s",
				elasticsearch.ESIndexName(i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion))
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
		For(&xjoin.XJoinPipeline{}).
		Complete(r)
}

func (i *ReconcileIteration) addFinalizer() error {
	if !utils.ContainsString(i.Instance.GetFinalizers(), xjoinpipelineFinalizer) {
		controllerutil.AddFinalizer(i.Instance, xjoinpipelineFinalizer)
		return i.Client.Update(context.TODO(), i.Instance)
	}

	return nil
}

func (i *ReconcileIteration) removeFinalizer() error {
	controllerutil.RemoveFinalizer(i.Instance, xjoinpipelineFinalizer)
	return i.Client.Update(context.TODO(), i.Instance)
}

func (i *ReconcileIteration) deleteStaleDependencies() (errors []error) {
	var (
		connectorsToKeep []string
		esIndicesToKeep  []string
		topicsToKeep     []string
	)

	if i.Instance.GetState() != xjoin.STATE_REMOVED && i.Instance.Status.PipelineVersion != "" {
		connectorsToKeep = append(connectorsToKeep, kafka.DebeziumConnectorName(i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion))
		connectorsToKeep = append(connectorsToKeep, kafka.ESConnectorName(i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion))
		esIndicesToKeep = append(esIndicesToKeep, elasticsearch.ESIndexName(i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion))
		topicsToKeep = append(topicsToKeep, kafka.TopicName(i.Instance.Status.PipelineVersion))
	}

	currentIndices, err := i.ESClient.GetCurrentIndicesWithAlias(i.parameters.ResourceNamePrefix.String())
	if err != nil {
		errors = append(errors, err)
	} else if currentIndices != nil && i.Instance.GetState() != xjoin.STATE_REMOVED {
		version := currentIndices[0][len("xjoin.inventory.hosts"):len(currentIndices[0])]
		connectorsToKeep = append(connectorsToKeep, kafka.DebeziumConnectorName(i.parameters.ResourceNamePrefix.String(), version))
		connectorsToKeep = append(connectorsToKeep, kafka.ESConnectorName(i.parameters.ResourceNamePrefix.String(), version))
		esIndicesToKeep = append(esIndicesToKeep, currentIndices...)
		topicsToKeep = append(topicsToKeep, kafka.TopicName(version))
	}

	//delete stale Kafka Connectors
	connectors, err := kafka.GetConnectorsForOwner(i.Client, i.Instance.Namespace, i.Instance.GetUIDString())
	if err != nil {
		errors = append(errors, err)
	} else {
		for _, connector := range connectors.Items {
			if !utils.ContainsString(connectorsToKeep, connector.GetName()) {
				i.Log.Info("Removing stale connector", "connector", connector.GetName())
				if err = kafka.DeleteConnector(i.Client, connector.GetName(), i.Instance.Namespace); err != nil {
					errors = append(errors, err)
				}
			}
		}
	}

	//delete stale ES indices
	indices, err := i.ESClient.ListIndices(i.parameters.ResourceNamePrefix.String())
	if err != nil {
		errors = append(errors, err)
	} else {
		for _, index := range indices {
			if !utils.ContainsString(esIndicesToKeep, index) {
				i.Log.Info("Removing stale index", "index", index)
				if err = i.ESClient.DeleteIndexByFullName(index); err != nil {
					errors = append(errors, err)
				}
			}
		}
	}

	//delete stale Kafka Topics
	topics, err := i.Kafka.ListTopicNames()
	if err != nil {
		errors = append(errors, err)
	} else {
		for _, topic := range topics {
			if !utils.ContainsString(topicsToKeep, topic) {
				i.Log.Info("Removing stale topic", "topic", topic)
				if err = i.Kafka.DeleteTopic(topic); err != nil {
					errors = append(errors, err)
				}
			}
		}
	}

	return
}

func (i *ReconcileIteration) createESIndex(pipelineVersion string) error {
	err := i.ESClient.CreateIndex(i.parameters.ResourceNamePrefix.String(), pipelineVersion)
	if err != nil {
		return err
	}
	return nil
}

func (i *ReconcileIteration) recreateAliasIfNeeded() (bool, error) {
	currentIndices, err := i.ESClient.GetCurrentIndicesWithAlias(i.parameters.ResourceNamePrefix.String())
	if err != nil {
		return false, err
	}

	validIndex := elasticsearch.ESIndexName(i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion)
	if currentIndices == nil || !utils.ContainsString(currentIndices, validIndex) {
		i.Log.Info("Updating alias", "index", validIndex)
		if err = i.ESClient.UpdateAliasByFullIndexName(i.parameters.ResourceNamePrefix.String(), validIndex); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

/*
 * Should be called when a refreshed pipeline failed to become valid.
 * This method will either keep the old invalid ES index "active" (i.e. used by the alias)
 * or update the alias to the new (also invalid) table.
 * None of these options a good one - this is about picking lesser evil
 */
func (i *ReconcileIteration) updateAliasIfHealthier() error {
	indices, err := i.ESClient.GetCurrentIndicesWithAlias(i.parameters.ResourceNamePrefix.String())

	if err != nil {
		return fmt.Errorf("Failed to determine active index %w", err)
	}

	if indices != nil {
		if len(indices) == 1 && indices[0] == elasticsearch.ESIndexName(
			i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion) {
			return nil // index is already active, nothing to do
		}

		// no need to close this as that's done in ReconcileIteration.Close()
		i.InventoryDb = database.NewBaseDatabase(database.DBParams{
			Host:     i.parameters.HBIDBHost.String(),
			User:     i.parameters.HBIDBUser.String(),
			Name:     i.parameters.HBIDBName.String(),
			Port:     i.parameters.HBIDBPort.String(),
			Password: i.parameters.HBIDBPassword.String(),
		})

		if err = i.InventoryDb.Connect(); err != nil {
			return err
		}

		hbiHostCount, err := i.InventoryDb.CountHosts()
		if err != nil {
			return fmt.Errorf("failed to get host count from inventory %w", err)
		}

		activeCount, err := i.ESClient.CountIndex("xjoin.inventory.hosts")
		if err != nil {
			return fmt.Errorf("failed to get host count from active index %w", err)
		}
		latestCount, err := i.ESClient.CountIndex(elasticsearch.ESIndexName(i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion))
		if err != nil {
			return fmt.Errorf("failed to get host count from latest index %w", err)
		}

		if utils.Abs(hbiHostCount-latestCount) > utils.Abs(hbiHostCount-activeCount) {
			return nil // the active table is healthier; do not update anything
		}
	}

	if err = i.ESClient.UpdateAliasByFullIndexName(
		i.parameters.ResourceNamePrefix.String(),
		elasticsearch.ESIndexName(i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion)); err != nil {
		return err
	}

	return nil
}

func (i *ReconcileIteration) checkForDeviation() (problem error, err error) {
	if i.Instance.Status.XJoinConfigVersion != i.parameters.ConfigMapVersion.String() {
		return fmt.Errorf("configMap changed. New version is %s", i.parameters.ConfigMapVersion), nil
	}

	//ES Index
	indexExists, err := i.ESClient.IndexExists(i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion)

	if err != nil {
		return nil, err
	} else if indexExists == false {
		return fmt.Errorf(
			"elasticsearch index %s not found",
			elasticsearch.ESIndexName(i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion)), nil
	}

	esConnectorName := kafka.ESConnectorName(i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion)
	problem, err = i.checkConnectorDeviation(esConnectorName, "es")
	if err != nil {
		return problem, err
	}

	debeziumConnectorName := kafka.DebeziumConnectorName(i.parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion)
	problem, err = i.checkConnectorDeviation(debeziumConnectorName, "debezium")
	if err != nil {
		return problem, err
	}
	return nil, nil
}

func (i *ReconcileIteration) checkConnectorDeviation(connectorName string, connectorType string) (problem error, err error) {
	connector, err := kafka.GetConnector(i.Client, connectorName, i.Instance.Namespace)
	if err != nil {
		if k8errors.IsNotFound(err) {
			return fmt.Errorf(
				"connector %s not found in %s", connectorName, i.Instance.Namespace), nil
		}

		return nil, err

	}

	if kafka.IsFailed(connector) {
		return fmt.Errorf("connector %s is in the FAILED state", connectorName), nil
	}

	if connector.GetLabels()[kafka.LabelStrimziCluster] != i.parameters.ConnectCluster.String() {
		return fmt.Errorf(
			"connectCluster changed from %s to %s",
			connector.GetLabels()[kafka.LabelStrimziCluster],
			i.parameters.ConnectCluster.String()), nil
	}

	/*
		if connector.GetLabels()[connect.LabelMaxAge] != strconv.FormatInt(i.parameters.ConnectorMaxAge, 10) {
			return fmt.Errorf(
				"maxAge changed from %s to %d",
				connector.GetLabels()[connect.LabelMaxAge],
				i.parameters.ConnectorMaxAge), nil
		}
	*/

	// compares the spec of the existing connector with the spec we would create if we were creating a new connector now
	newConnector, err := i.createDryConnectorByType(connectorType)
	if err != nil {
		return nil, err
	}

	currentConnectorConfig, _, err1 := unstructured.NestedMap(connector.UnstructuredContent(), "spec", "parameters")
	newConnectorConfig, _, err2 := unstructured.NestedMap(newConnector.UnstructuredContent(), "spec", "parameters")

	if err1 == nil && err2 == nil {
		diff := cmp.Diff(currentConnectorConfig, newConnectorConfig, NumberNormalizer)

		if len(diff) > 0 {
			return fmt.Errorf("connector configuration has changed: %s", diff), nil
		}
	}

	return nil, nil
}

func (i *ReconcileIteration) createDryConnectorByType(conType string) (*unstructured.Unstructured, error) {
	if conType == "es" {
		return i.Kafka.CreateESConnector(i.parameters, i.Instance.Status.PipelineVersion, true)
	} else if conType == "debezium" {
		return i.Kafka.CreateDebeziumConnector(i.parameters, i.Instance.Status.PipelineVersion, true)
	} else {
		return nil, errors.New("invalid param. Must be one of [es, debezium]")
	}
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
