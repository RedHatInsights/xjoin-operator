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
	"github.com/redhatinsights/xjoin-operator/controllers/connect"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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

var log = logf.Log.WithName("controller_xjoinpipeline")

func (r *XJoinPipelineReconciler) setup(reqLogger logr.Logger, request ctrl.Request) (ReconcileIteration, error) {

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
		GetRequeueInterval: func(Instance *ReconcileIteration) int64 {
			return i.config.StandardInterval
		},
		Recorder: r.Recorder,
	}

	if err = i.parseConfig(); err != nil {
		return i, err
	}

	es, err := elasticsearch.NewElasticSearch(
		i.config.ElasticSearchURL, i.config.ElasticSearchUsername, i.config.ElasticSearchPassword)
	if err != nil {
		return i, err
	}
	i.ESClient = es

	if i.HBIDBParams, err = config.LoadSecret(i.Client, i.Instance.Namespace, "host-inventory-db"); err != nil {
		return i, err
	}

	return i, nil
}

// +kubebuilder:rbac:groups=xjoin.cloud.redhat.com,resources=xjoinpipelines;xjoinpipelines/status;xjoinpipelines/finalizers,verbs=*
// +kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkaconnectors;kafkaconnectors/finalizers,verbs=*
// +kubebuilder:rbac:groups="",resources=configmaps;secrets,verbs=get;list;watch

func (r *XJoinPipelineReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	reqLogger := log.WithValues("Pipeline", request.Name, "Namespace", request.Namespace)
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

		i.Instance.Status.XJoinConfigVersion = i.config.ConfigMapVersion

		pipelineVersion := fmt.Sprintf("%s", strconv.FormatInt(time.Now().UnixNano(), 10))
		if err := i.Instance.TransitionToInitialSync(pipelineVersion); err != nil {
			i.error(err, "Error transitioning to Initial Sync")
			return reconcile.Result{}, err
		}
		i.probeStartingInitialSync()

		err := i.ESClient.CreateIndex(pipelineVersion)
		if err != nil {
			i.error(err, "Error creating ElasticSearch index")
			return reconcile.Result{}, err
		}

		_, err = i.createDebeziumConnector(pipelineVersion, false)
		if err != nil {
			i.error(err, "Error creating debezium connector")
			return reconcile.Result{}, err
		}
		_, err = i.createESConnector(pipelineVersion, false)
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
				elasticsearch.ESIndexName(i.Instance.Status.PipelineVersion))
		}

		return i.updateStatusAndRequeue()
	}

	// invalid pipeline - either STATE_INITIAL_SYNC or STATE_INVALID
	if i.Instance.GetValid() == metav1.ConditionFalse {
		if i.Instance.Status.ValidationFailedCount >= i.getValidationConfig().AttemptsThreshold {

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
	)

	if i.Instance.GetState() != xjoin.STATE_REMOVED && i.Instance.Status.PipelineVersion != "" {
		connectorsToKeep = append(connectorsToKeep, xjoin.DebeziumConnectorName(i.Instance.Status.PipelineVersion))
		connectorsToKeep = append(connectorsToKeep, xjoin.ESConnectorName(i.Instance.Status.PipelineVersion))
		esIndicesToKeep = append(esIndicesToKeep, elasticsearch.ESIndexName(i.Instance.Status.PipelineVersion))
	}

	currentIndices, err := i.ESClient.GetCurrentIndicesWithAlias()
	if err != nil {
		errors = append(errors, err)
	} else if currentIndices != nil && i.Instance.GetState() != xjoin.STATE_REMOVED {
		version := currentIndices[0][len("xjoin.inventory.hosts"):len(currentIndices[0])]
		connectorsToKeep = append(connectorsToKeep, xjoin.DebeziumConnectorName(version))
		connectorsToKeep = append(connectorsToKeep, xjoin.ESConnectorName(version))
		esIndicesToKeep = append(esIndicesToKeep, currentIndices...)
	}

	connectors, err := connect.GetConnectorsForOwner(i.Client, i.Instance.Namespace, i.Instance.GetUIDString())
	if err != nil {
		errors = append(errors, err)
	} else {
		for _, connector := range connectors.Items {
			if !utils.ContainsString(connectorsToKeep, connector.GetName()) {
				i.Log.Info("Removing stale connector", "connector", connector.GetName())
				if err = connect.DeleteConnector(i.Client, connector.GetName(), i.Instance.Namespace); err != nil {
					errors = append(errors, err)
				}
			}
		}
	}

	//delete stale ES indices
	indices, err := i.ESClient.ListIndices()
	if err != nil {
		errors = append(errors, err)
	} else {
		for _, index := range indices {
			if !utils.ContainsString(esIndicesToKeep, index) {
				i.Log.Info("Removing stale index", "index", index)
				if err = i.ESClient.DeleteIndex(index); err != nil {
					errors = append(errors, err)
				}
			}
		}
	}

	return
}

func (i *ReconcileIteration) createDebeziumConnector(pipelineVersion string, dryRun bool) (*unstructured.Unstructured, error) {
	var debeziumConfig = connect.DebeziumConnectorConfiguration{
		Cluster:     i.config.ConnectCluster,
		Template:    i.config.DebeziumConnectorTemplate,
		HBIDBParams: i.HBIDBParams,
		Version:     pipelineVersion,
	}

	return connect.CreateDebeziumConnector(
		i.Client, i.Instance.Namespace, debeziumConfig, i.Instance, i.Scheme, dryRun)
}

func (i *ReconcileIteration) createESConnector(pipelineVersion string, dryRun bool) (*unstructured.Unstructured, error) {
	var esConfig = connect.ElasticSearchConnectorConfiguration{
		Cluster:               i.config.ConnectCluster,
		Template:              i.config.ElasticSearchConnectorTemplate,
		ElasticSearchURL:      i.config.ElasticSearchURL,
		ElasticSearchUsername: i.config.ElasticSearchUsername,
		ElasticSearchPassword: i.config.ElasticSearchPassword,
		Version:               pipelineVersion,
		MaxAge:                i.config.ConnectorMaxAge,
	}

	return connect.CreateESConnector(
		i.Client, i.Instance.Namespace, esConfig, i.Instance, i.Scheme, dryRun)
}

func (i *ReconcileIteration) createESIndex(pipelineVersion string) error {
	err := i.ESClient.CreateIndex(pipelineVersion)
	if err != nil {
		return err
	}
	return nil
}

func (i *ReconcileIteration) recreateAliasIfNeeded() (bool, error) {
	currentIndices, err := i.ESClient.GetCurrentIndicesWithAlias()
	if err != nil {
		return false, err
	}

	validIndex := elasticsearch.ESIndexName(i.Instance.Status.PipelineVersion)
	if currentIndices == nil || !utils.ContainsString(currentIndices, validIndex) {
		i.Log.Info("Updating alias", "index", validIndex)
		if err = i.ESClient.UpdateAlias(validIndex); err != nil {
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
	indices, err := i.ESClient.GetCurrentIndicesWithAlias()

	if err != nil {
		return fmt.Errorf("Failed to determine active index %w", err)
	}

	if indices != nil {
		if len(indices) == 1 && indices[0] == elasticsearch.ESIndexName(i.Instance.Status.PipelineVersion) {
			return nil // index is already active, nothing to do
		}

		// no need to close this as that's done in ReconcileIteration.Close()
		i.InventoryDb = database.NewBaseDatabase(&i.HBIDBParams)

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
		latestCount, err := i.ESClient.CountIndex(elasticsearch.ESIndexName(i.Instance.Status.PipelineVersion))
		if err != nil {
			return fmt.Errorf("failed to get host count from latest index %w", err)
		}

		if utils.Abs(hbiHostCount-latestCount) > utils.Abs(hbiHostCount-activeCount) {
			return nil // the active table is healthier; do not update anything
		}
	}

	if err = i.ESClient.UpdateAlias(elasticsearch.ESIndexName(i.Instance.Status.PipelineVersion)); err != nil {
		return err
	}

	return nil
}

func (i *ReconcileIteration) checkForDeviation() (problem error, err error) {
	if i.Instance.Status.XJoinConfigVersion != i.config.ConfigMapVersion {
		return fmt.Errorf("ConfigMap changed. New version is %s", i.config.ConfigMapVersion), nil
	}

	//ES Index
	indexExists, err := i.ESClient.IndexExists(elasticsearch.ESIndexName(i.Instance.Status.PipelineVersion))

	if err != nil {
		return nil, err
	} else if indexExists == false {
		return fmt.Errorf(
			"elasticsearch index %s not found",
			elasticsearch.ESIndexName(i.Instance.Status.PipelineVersion)), nil
	}

	esConnectorName := xjoin.ESConnectorName(i.Instance.Status.PipelineVersion)
	problem, err = i.checkConnectorDeviation(esConnectorName, "es")
	if err != nil {
		return problem, err
	}

	debeziumConnectorName := xjoin.DebeziumConnectorName(i.Instance.Status.PipelineVersion)
	problem, err = i.checkConnectorDeviation(debeziumConnectorName, "debezium")
	if err != nil {
		return problem, err
	}
	return nil, nil
}

func (i *ReconcileIteration) checkConnectorDeviation(connectorName string, connectorType string) (problem error, err error) {
	connector, err := connect.GetConnector(i.Client, connectorName, i.Instance.Namespace)
	if err != nil {
		if k8errors.IsNotFound(err) {
			return fmt.Errorf(
				"connector %s not found in %s", connectorName, i.Instance.Namespace), nil
		}

		return nil, err

	}

	if connect.IsFailed(connector) {
		return fmt.Errorf("connector %s is in the FAILED state", connectorName), nil
	}

	if connector.GetLabels()[connect.LabelStrimziCluster] != i.config.ConnectCluster {
		return fmt.Errorf(
			"connectCluster changed from %s to %s",
			connector.GetLabels()[connect.LabelStrimziCluster],
			i.config.ConnectCluster), nil
	}

	/*
		if connector.GetLabels()[connect.LabelMaxAge] != strconv.FormatInt(i.config.ConnectorMaxAge, 10) {
			return fmt.Errorf(
				"maxAge changed from %s to %d",
				connector.GetLabels()[connect.LabelMaxAge],
				i.config.ConnectorMaxAge), nil
		}
	*/

	// compares the spec of the existing connector with the spec we would create if we were creating a new connector now
	newConnector, err := i.createDryConnectorByType(connectorType)
	if err != nil {
		return nil, err
	}

	currentConnectorConfig, _, err1 := unstructured.NestedMap(connector.UnstructuredContent(), "spec", "config")
	newConnectorConfig, _, err2 := unstructured.NestedMap(newConnector.UnstructuredContent(), "spec", "config")

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
		return i.createESConnector(i.Instance.Status.PipelineVersion, true)
	} else if conType == "debezium" {
		return i.createDebeziumConnector(i.Instance.Status.PipelineVersion, true)
	} else {
		return nil, errors.New("invalid param. Must be one of [es, debezium]")
	}
}

func NewXJoinReconciler(client client.Client,
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
