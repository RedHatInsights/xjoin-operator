package pipeline

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	corev1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"time"
)

// XJoinPipelineReconciler reconciles a XJoinPipeline object
const xjoinpipelineFinalizer = "finalizer.xjoin.cloud.redhat.com"

type ReconcileIteration struct {
	Instance *xjoin.XJoinPipeline
	// Do not alter this copy
	// Used for tracking of whether Reconcile actually changed the state or not
	OriginalInstance *xjoin.XJoinPipeline

	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Log      logger.Log
	Client   client.Client
	Now      string

	Parameters config.Parameters

	ESClient        *elasticsearch.ElasticSearch
	Kafka           kafka.Kafka
	KafkaTopics     kafka.Topics
	KafkaConnectors kafka.Connectors
	InventoryDb     *database.Database

	GetRequeueInterval func(i *ReconcileIteration) (result int)
}

func (i *ReconcileIteration) Close() {
	if i.InventoryDb != nil {
		i.InventoryDb.Close()
	}
}

// logs the error and produces an error log message
func (i *ReconcileIteration) Error(err error, prefixes ...string) {
	msg := err.Error()

	if len(prefixes) > 0 {
		prefix := strings.Join(prefixes[:], ", ")
		msg = fmt.Sprintf("%s: %s", prefix, msg)
	}

	i.Log.Error(err, msg)

	i.EventWarning("Failed", msg)
}

func (i *ReconcileIteration) EventNormal(reason, messageFmt string, args ...interface{}) {
	i.Recorder.Eventf(i.Instance, corev1.EventTypeNormal, reason, messageFmt, args...)
}

func (i *ReconcileIteration) EventWarning(reason, messageFmt string, args ...interface{}) {
	i.Recorder.Eventf(i.Instance, corev1.EventTypeWarning, reason, messageFmt, args...)
}

func (i *ReconcileIteration) Debug(message string, keysAndValues ...interface{}) {
	i.Log.Debug(message, keysAndValues...)
}

func (i *ReconcileIteration) SetActiveResources() {
	i.Instance.Status.ActivePipelineVersion = i.Instance.Status.PipelineVersion
	i.Instance.Status.ActiveAliasName = i.ESClient.AliasName()
	i.Instance.Status.ActiveDebeziumConnectorName = i.KafkaConnectors.DebeziumConnectorName(i.Instance.Status.PipelineVersion)
	i.Instance.Status.ActiveESConnectorName = i.KafkaConnectors.ESConnectorName(i.Instance.Status.PipelineVersion)
	i.Instance.Status.ActiveESPipelineName = i.ESClient.ESPipelineName(i.Instance.Status.PipelineVersion)
	i.Instance.Status.ActiveTopicName = i.KafkaTopics.TopicName(i.Instance.Status.PipelineVersion)
	i.Instance.Status.ActiveReplicationSlotName =
		database.ReplicationSlotName(i.Parameters.ResourceNamePrefix.String(), i.Instance.Status.PipelineVersion)
}

func (i *ReconcileIteration) UpdateStatusAndRequeue() (reconcile.Result, error) {
	i.Log.Info("Checking status")
	// Update Status.ActiveIndexName to reflect the active index regardless of what happened in this Reconcile() invocation
	currentIndices, err := i.ESClient.GetCurrentIndicesWithAlias(i.Instance.Status.ActiveAliasName)
	if err != nil {
		i.Log.Error(err, "Unable to get current index with alias")
		return reconcile.Result{}, err
	}

	if len(currentIndices) == 0 {
		i.Instance.Status.ActiveIndexName = ""
	} else {
		i.Instance.Status.ActiveIndexName = currentIndices[0]
	}

	// Only issue status update if Reconcile actually modified Status
	// This prevents write conflicts between the controllers
	if !cmp.Equal(i.Instance.Status, i.OriginalInstance.Status) {
		i.Log.Info("Updating status")
		i.Debug("Updating status")

		ctx, cancel := utils.DefaultContext()
		defer cancel()

		if err := i.Client.Status().Update(ctx, i.Instance); err != nil {
			if k8errors.IsConflict(err) {
				i.Log.Error(err, "Status conflict")
				return reconcile.Result{}, err
			}

			i.Error(err, "Error updating pipeline status")
			return reconcile.Result{}, err
		}
	}

	delay := time.Second * time.Duration(i.GetRequeueInterval(i))
	i.Debug("RequeueAfter", "delay", delay)
	return reconcile.Result{RequeueAfter: delay}, nil
}

func (i *ReconcileIteration) GetValidationInterval() int {
	if i.Instance.Status.InitialSyncInProgress {
		return i.Parameters.ValidationInitInterval.Int()
	}

	return i.Parameters.ValidationInterval.Int()
}

func (i *ReconcileIteration) GetValidationAttemptsThreshold() int {
	if i.Instance.Status.InitialSyncInProgress {
		return i.Parameters.ValidationInitAttemptsThreshold.Int()
	}

	return i.Parameters.ValidationAttemptsThreshold.Int()
}

func (i *ReconcileIteration) GetValidationPercentageThreshold() int {
	if i.Instance.Status.InitialSyncInProgress {
		return i.Parameters.ValidationInitPercentageThreshold.Int()
	}

	return i.Parameters.ValidationPercentageThreshold.Int()
}

func (i *ReconcileIteration) AddFinalizer() error {
	if !utils.ContainsString(i.Instance.GetFinalizers(), xjoinpipelineFinalizer) {
		controllerutil.AddFinalizer(i.Instance, xjoinpipelineFinalizer)

		ctx, cancel := utils.DefaultContext()
		defer cancel()
		return i.Client.Update(ctx, i.Instance)
	}

	return nil
}

func (i *ReconcileIteration) RemoveFinalizer() error {
	controllerutil.RemoveFinalizer(i.Instance, xjoinpipelineFinalizer)
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	return i.Client.Update(ctx, i.Instance)
}

func (i *ReconcileIteration) IsXJoinResource(resourceName string) bool {
	var response bool

	if strings.Index(resourceName, "xjoin-connect-config") == 0 {
		response = false
	} else if strings.Index(resourceName, "xjoin-connect-offsets") == 0 {
		response = false
	} else if strings.Index(resourceName, "xjoin-connect-status") == 0 {
		response = false
	} else if strings.Index(resourceName, i.Parameters.ResourceNamePrefix.String()) == 0 {
		response = true
	} else if strings.Index(resourceName, database.ReplicationSlotPrefix(i.Parameters.ResourceNamePrefix.String())) == 0 {
		response = true
	}

	return response
}

func (i *ReconcileIteration) DeleteStaleDependencies() (errors []error) {
	i.Log.Debug("Deleting stale dependencies")

	var (
		connectorsToKeep       []string
		esIndicesToKeep        []string
		topicsToKeep           []string
		replicationSlotsToKeep []string
		esPipelinesToKeep      []string
	)

	resourceNamePrefix := i.Parameters.ResourceNamePrefix.String()

	connectorsToKeep = append(connectorsToKeep, i.Instance.Status.ActiveDebeziumConnectorName)
	connectorsToKeep = append(connectorsToKeep, i.Instance.Status.ActiveESConnectorName)
	esPipelinesToKeep = append(esPipelinesToKeep, i.Instance.Status.ActiveESPipelineName)
	esIndicesToKeep = append(esIndicesToKeep, i.Instance.Status.ActiveIndexName)
	topicsToKeep = append(topicsToKeep, i.Instance.Status.ActiveTopicName)
	replicationSlotsToKeep = append(replicationSlotsToKeep, i.Instance.Status.ActiveReplicationSlotName)

	var staleResources []string

	//keep the in progress pipeline's resources and the active resources
	if i.Instance.GetState() != xjoin.STATE_REMOVED && i.Instance.Status.PipelineVersion != "" {
		connectorsToKeep = append(connectorsToKeep, i.KafkaConnectors.DebeziumConnectorName(i.Instance.Status.PipelineVersion))
		connectorsToKeep = append(connectorsToKeep, i.KafkaConnectors.ESConnectorName(i.Instance.Status.PipelineVersion))
		esPipelinesToKeep = append(esPipelinesToKeep, i.ESClient.ESPipelineName(i.Instance.Status.PipelineVersion))
		esIndicesToKeep = append(esIndicesToKeep, i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion))
		topicsToKeep = append(topicsToKeep, i.KafkaTopics.TopicName(i.Instance.Status.PipelineVersion))
		replicationSlotsToKeep = append(replicationSlotsToKeep, database.ReplicationSlotName(
			resourceNamePrefix, i.Instance.Status.PipelineVersion))
	}

	i.Log.Debug("ConnectorsToKeep", "connectors", connectorsToKeep)
	i.Log.Debug("ESPipelinesToKeep", "pipelines", esPipelinesToKeep)
	i.Log.Debug("ESIndicesToKeep", "indices", esIndicesToKeep)
	i.Log.Info("TopicsToKeep", "topics", topicsToKeep)
	i.Log.Debug("ReplicationSlotsToKeep", "slots", replicationSlotsToKeep)

	//delete stale Kafka Connectors
	connectors, err := i.Kafka.ListConnectors()
	if err != nil {
		errors = append(errors, err)
	} else {
		for _, connector := range connectors.Items {
			i.Log.Debug("AllConnectors", "connector", connector.GetName())
			if !utils.ContainsString(connectorsToKeep, connector.GetName()) && i.IsXJoinResource(connector.GetName()) {
				i.Log.Info("Removing stale connector", "connector", connector.GetName())
				if err = i.Kafka.DeleteConnector(connector.GetName()); err != nil {
					staleResources = append(staleResources, "KafkaConnector/"+connector.GetName())
					errors = append(errors, err)
				}
			}
		}
	}

	//delete stale ES pipelines
	esPipelines, err := i.ESClient.ListESPipelines()
	if err != nil {
		errors = append(errors, err)
	} else {
		i.Log.Debug("AllESPipelines", "pipelines", esPipelines)
		for _, esPipeline := range esPipelines {
			if !utils.ContainsString(esPipelinesToKeep, esPipeline) && i.IsXJoinResource(esPipeline) {
				i.Log.Info("Removing stale es pipeline", "esPipeline", esPipeline)
				if err = i.ESClient.DeleteESPipelineByFullName(esPipeline); err != nil {
					staleResources = append(staleResources, "ESPipeline/"+esPipeline)
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
		i.Log.Debug("AllIndices", "indices", indices)
		for _, index := range indices {
			if !utils.ContainsString(esIndicesToKeep, index) && i.IsXJoinResource(index) {
				i.Log.Info("Removing stale index", "index", index)
				if err = i.ESClient.DeleteIndexByFullName(index); err != nil {
					staleResources = append(staleResources, "ESIndex/"+index)
					errors = append(errors, err)
				}
			}
		}
	}

	//delete stale Kafka Topics
	topics, err := i.KafkaTopics.ListTopicNamesForPrefix(resourceNamePrefix)
	if err != nil {
		errors = append(errors, err)
	} else {
		i.Log.Debug("AllTopics", "topics", topics)
		for _, topic := range topics {
			if !utils.ContainsString(topicsToKeep, topic) && i.IsXJoinResource(topic) {
				i.Log.Info("Removing stale topic", "topic", topic)
				if err = i.KafkaTopics.DeleteTopic(topic); err != nil {
					staleResources = append(staleResources, "KafkaTopic/"+topic)
					errors = append(errors, err)
				}
			}
		}
	}

	//delete stale replication slots
	slots, err := i.InventoryDb.ListReplicationSlots(resourceNamePrefix)
	if err != nil {
		errors = append(errors, err)
	} else {
		i.Log.Debug("AllReplicationSlots", "slots", slots)
		for _, slot := range slots {
			if !utils.ContainsString(replicationSlotsToKeep, slot) && i.IsXJoinResource(slot) {
				i.Log.Info("Removing stale replication slot", "slot", slot)
				if err = i.InventoryDb.RemoveReplicationSlot(slot); err != nil {
					staleResources = append(staleResources, "ReplicationSlot/"+slot)
					errors = append(errors, err)
				}
			}
		}
	}

	metrics.StaleResourceCount(staleResources)

	return
}

func (i *ReconcileIteration) RecreateAliasIfNeeded() (bool, error) {
	currentIndices, err := i.ESClient.GetCurrentIndicesWithAlias(i.Instance.Status.ActiveAliasName)
	if err != nil {
		return false, err
	}

	newIndex := i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion)
	if currentIndices == nil || !utils.ContainsString(currentIndices, newIndex) {
		i.Log.Info("Updating alias", "index", newIndex)
		if err = i.ESClient.UpdateAliasByFullIndexName(i.ESClient.AliasName(), newIndex); err != nil {
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
func (i *ReconcileIteration) UpdateAliasIfHealthier() error {
	indices, err := i.ESClient.GetCurrentIndicesWithAlias(i.Instance.Status.ActiveAliasName)

	if err != nil {
		return fmt.Errorf("Failed to determine active index %w", err)
	}

	if indices != nil {
		if len(indices) == 1 && indices[0] == i.Instance.Status.ActiveIndexName {
			return nil // index is already active, nothing to do
		}

		// no need to close this as that's done in ReconcileIteration.Close()
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
			return err
		}

		hbiHostCount, err := i.InventoryDb.CountHosts()
		if err != nil {
			return fmt.Errorf("failed to get host count from inventory %w", err)
		}

		activeCount, err := i.ESClient.CountIndex(i.Instance.Status.ActiveIndexName)
		if err != nil {
			return fmt.Errorf("failed to get host count from active index %w", err)
		}
		latestCount, err := i.ESClient.CountIndex(i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion))
		if err != nil {
			return fmt.Errorf("failed to get host count from latest index %w", err)
		}

		if utils.Abs(hbiHostCount-latestCount) > utils.Abs(hbiHostCount-activeCount) {
			return nil // the active table is healthier; do not update anything
		}
	}

	//don't remove the jenkins managed alias until the operator pipeline is healthy
	currIndices, err := i.ESClient.GetCurrentIndicesWithAlias("xjoin.inventory.hosts")
	if err != nil {
		return err
	}
	if !utils.ContainsString(currIndices, "xjoin.inventory.hosts."+i.Parameters.JenkinsManagedVersion.String()) {
		if err = i.ESClient.UpdateAliasByFullIndexName(
			i.ESClient.AliasName(),
			i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion)); err != nil {
			return err
		}
	}

	return nil
}

func (i *ReconcileIteration) CheckForDeviation() (problem error, err error) {
	//Configmap/secrets
	if i.Instance.Status.XJoinConfigVersion != i.Parameters.ConfigMapVersion.String() {
		return fmt.Errorf("configMap changed. New version is %s",
			i.Parameters.ConfigMapVersion.String()), nil
	}

	if i.Instance.Status.ElasticSearchSecretVersion != i.Parameters.ElasticSearchSecretVersion.String() {
		return fmt.Errorf("elasticsesarch secret changed. New version is %s",
			i.Parameters.ElasticSearchSecretVersion.String()), nil
	}

	if i.Instance.Status.HBIDBSecretVersion != i.Parameters.HBIDBSecretVersion.String() {
		return fmt.Errorf("hbidbsecret changed. New version is %s",
			i.Parameters.HBIDBSecretVersion.String()), nil
	}

	//ES Index
	problem, err = i.CheckESIndexDeviation()
	if err != nil || problem != nil {
		return problem, err
	}

	//ES Pipeline
	problem, err = i.CheckESPipelineDeviation()
	if err != nil || problem != nil {
		return problem, err
	}

	//Connectors
	problem, err = i.CheckConnectorDeviation(
		i.KafkaConnectors.ESConnectorName(i.Instance.Status.PipelineVersion), "es")
	if err != nil || problem != nil {
		return problem, err
	}

	problem, err = i.CheckConnectorDeviation(
		i.KafkaConnectors.DebeziumConnectorName(i.Instance.Status.PipelineVersion), "debezium")
	if err != nil || problem != nil {
		return problem, err
	}

	//Topic
	problem, err = i.CheckTopicDeviation()
	if err != nil || problem != nil {
		return problem, err
	}

	return nil, nil
}

func (i *ReconcileIteration) CheckESPipelineDeviation() (problem error, err error) {
	i.Log.Info("Checking espipeline deviation")
	if i.Instance.Status.PipelineVersion == "" {
		return nil, nil
	}

	pipelineName := i.ESClient.ESPipelineName(i.Instance.Status.PipelineVersion)
	esPipelineExists, err := i.ESClient.ESPipelineExists(i.Instance.Status.PipelineVersion)

	if err != nil {
		return nil, err
	} else if !esPipelineExists {
		return fmt.Errorf("elasticsearch pipeline %s not found", pipelineName), nil
	}

	return nil, nil
}

func (i *ReconcileIteration) CheckESIndexDeviation() (problem error, err error) {
	i.Log.Info("Checking es index deviation")
	if i.Instance.Status.PipelineVersion == "" {
		return nil, nil
	}

	indexName := i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion)
	indexExists, err := i.ESClient.IndexExists(indexName)

	if err != nil {
		return nil, err
	} else if !indexExists {
		return fmt.Errorf(
			"elasticsearch index %s not found",
			indexName), nil
	}

	return nil, nil
}

func (i *ReconcileIteration) CheckTopicDeviation() (problem error, err error) {
	i.Log.Info("Checking topic deviation")
	if i.Instance.Status.PipelineVersion == "" {
		return nil, nil
	}

	return i.KafkaTopics.CheckDeviation(i.Instance.Status.PipelineVersion)
}

func (i *ReconcileIteration) CheckConnectorDeviation(connectorName string, connectorType string) (problem error, err error) {
	i.Log.Info("Checking connector deviation. name: " + connectorName + " type: " + connectorType)

	if connectorName == "" {
		return nil, nil
	}

	i.Log.Info("Getting connector")
	connector, err := i.Kafka.GetConnector(connectorName)
	if err != nil {
		if k8errors.IsNotFound(err) {
			return fmt.Errorf(
				"connector %s not found in %s", connectorName, i.Instance.Namespace), nil
		}
		return nil, err
	}

	i.Log.Info("Checking if connector is failed")
	isFailed, err := i.KafkaConnectors.IsFailed(connectorName)
	if err != nil {
		return nil, err
	}

	if isFailed {
		i.Log.Warn("Connector is failed, restarting it.", "connector", connectorName)
		err = i.KafkaConnectors.RestartConnector(connectorName)
		if err != nil {
			return fmt.Errorf("connector %s is in the FAILED state", connectorName), nil
		}
	}

	i.Log.Info("Checking connector cluster label")
	if connector.GetLabels()[kafka.LabelStrimziCluster] != i.Parameters.ConnectCluster.String() {
		return fmt.Errorf(
			"connectCluster changed from %s to %s",
			connector.GetLabels()[kafka.LabelStrimziCluster],
			i.Parameters.ConnectCluster.String()), nil
	}

	i.Log.Info("Checking connector spec")
	// compares the spec of the existing connector with the spec we would create if we were creating a new connector now
	newConnector, err := i.KafkaConnectors.CreateDryConnectorByType(connectorType, i.Instance.Status.PipelineVersion)
	if err != nil {
		return nil, err
	}

	currentConnectorConfig, _, err1 := unstructured.NestedMap(connector.UnstructuredContent(), "spec", "config")
	newConnectorConfig, _, err2 := unstructured.NestedMap(newConnector.UnstructuredContent(), "spec", "config")

	if err1 == nil && err2 == nil {
		diff := cmp.Diff(currentConnectorConfig, newConnectorConfig, utils.NumberNormalizer)

		if len(diff) > 0 {
			return fmt.Errorf("connector configuration has changed: %s", diff), nil
		}
	}

	return nil, nil
}

func (i *ReconcileIteration) DeleteResourceForPipeline(version string) error {
	err := i.KafkaTopics.DeleteTopicByPipelineVersion(version)
	if err != nil {
		i.Error(err, "Error deleting topic")
		return err
	}

	err = i.KafkaConnectors.DeleteConnectorsForPipelineVersion(version)
	if err != nil {
		i.Error(err, "Error deleting connectors")
		return err
	}

	err = i.InventoryDb.RemoveReplicationSlotsForPipelineVersion(version)
	if err != nil {
		i.Error(err, "Error removing replication slots")
		return err
	}

	err = i.ESClient.DeleteIndex(version)
	if err != nil {
		i.Error(err, "Error removing ES indices")
		return err
	}

	err = i.ESClient.DeleteESPipelineByVersion(version)
	if err != nil {
		i.Error(err, "Error deleting ES pipeline")
		return err
	}

	return nil
}
