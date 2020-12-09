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
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/connect"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strconv"
	"time"

	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
)

// XJoinPipelineReconciler reconciles a XJoinPipeline object
type XJoinPipelineReconciler struct {
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
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

		_, err = i.createDebeziumConnector(pipelineVersion)
		if err != nil {
			i.error(err, "Error creating debezium connector")
			return reconcile.Result{}, err
		}
		_, err = i.createESConnector(pipelineVersion)
		if err != nil {
			i.error(err, "Error creating ES connector")
			return reconcile.Result{}, err
		}

		i.Log.Info("Transitioning to InitialSync")
		return i.updateStatusAndRequeue()
	}

	return ctrl.Result{}, nil
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

func (i *ReconcileIteration) createDebeziumConnector(pipelineVersion string) (*unstructured.Unstructured, error) {
	var debeziumConfig = connect.DebeziumConnectorConfiguration{
		Cluster:     i.config.ConnectCluster,
		Template:    i.config.DebeziumConnectorTemplate,
		HBIDBParams: i.HBIDBParams,
		Version:     pipelineVersion,
	}

	return connect.CreateDebeziumConnector(
		i.Client, i.Instance.Namespace, debeziumConfig, i.Instance, i.Scheme, false)
}

func (i *ReconcileIteration) createESConnector(pipelineVersion string) (*unstructured.Unstructured, error) {
	var esConfig = connect.ElasticSearchConnectorConfiguration{
		Cluster:               i.config.ConnectCluster,
		Template:              i.config.ElasticSearchConnectorTemplate,
		ElasticSearchURL:      i.config.ElasticSearchURL,
		ElasticSearchUsername: i.config.ElasticSearchUsername,
		ElasticSearchPassword: i.config.ElasticSearchPassword,
		Version:               pipelineVersion,
	}

	return connect.CreateESConnector(
		i.Client, i.Instance.Namespace, esConfig, i.Instance, i.Scheme, false)
}

func (i *ReconcileIteration) createESIndex(pipelineVersion string) error {
	err := i.ESClient.CreateIndex(pipelineVersion)
	if err != nil {
		return err
	}
	return nil
}
