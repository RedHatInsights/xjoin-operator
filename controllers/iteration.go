package controllers

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
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

	parameters config.Parameters

	ESClient    *elasticsearch.ElasticSearch
	Kafka       kafka.Kafka
	InventoryDb *database.Database

	GetRequeueInterval func(i *ReconcileIteration) (result int)
}

func (i *ReconcileIteration) Close() {
	if i.InventoryDb != nil {
		i.InventoryDb.Close()
	}
}

// logs the error and produces an error log message
func (i *ReconcileIteration) error(err error, prefixes ...string) {
	msg := err.Error()

	if len(prefixes) > 0 {
		prefix := strings.Join(prefixes[:], ", ")
		msg = fmt.Sprintf("%s: %s", prefix, msg)
	}

	i.Log.Error(err, msg)

	i.eventWarning("Failed", msg)
}

func (i *ReconcileIteration) eventNormal(reason, messageFmt string, args ...interface{}) {
	i.Recorder.Eventf(i.Instance, corev1.EventTypeNormal, reason, messageFmt, args...)
}

func (i *ReconcileIteration) eventWarning(reason, messageFmt string, args ...interface{}) {
	i.Recorder.Eventf(i.Instance, corev1.EventTypeWarning, reason, messageFmt, args...)
}

func (i *ReconcileIteration) debug(message string, keysAndValues ...interface{}) {
	i.Log.Debug(message, keysAndValues...)
}

func (i *ReconcileIteration) setActiveResources() {
	i.Instance.Status.ActiveAliasName = i.parameters.ResourceNamePrefix.String()
	i.Instance.Status.ActiveDebeziumConnectorName = i.Kafka.DebeziumConnectorName(i.Instance.Status.PipelineVersion)
	i.Instance.Status.ActiveESConnectorName = i.Kafka.ESConnectorName(i.Instance.Status.PipelineVersion)
	i.Instance.Status.ActiveTopicName = i.Kafka.TopicName(i.Instance.Status.PipelineVersion)
}

func (i *ReconcileIteration) updateStatusAndRequeue() (reconcile.Result, error) {
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
		i.debug("Updating status")

		if err := i.Client.Status().Update(context.TODO(), i.Instance); err != nil {
			if k8errors.IsConflict(err) {
				i.Log.Error(err, "Status conflict")
				return reconcile.Result{}, err
			}

			i.error(err, "Error updating pipeline status")
			return reconcile.Result{}, err
		}
	}

	delay := time.Second * time.Duration(i.GetRequeueInterval(i))
	i.debug("RequeueAfter", "delay", delay)
	return reconcile.Result{RequeueAfter: delay}, nil
}

func (i *ReconcileIteration) getValidationInterval() int {
	if i.Instance.Status.InitialSyncInProgress == true {
		return i.parameters.ValidationInitInterval.Int()
	}

	return i.parameters.ValidationInterval.Int()
}

func (i *ReconcileIteration) getValidationAttemptsThreshold() int {
	if i.Instance.Status.InitialSyncInProgress == true {
		return i.parameters.ValidationInitAttemptsThreshold.Int()
	}

	return i.parameters.ValidationAttemptsThreshold.Int()
}

func (i *ReconcileIteration) getValidationPercentageThreshold() int {
	if i.Instance.Status.InitialSyncInProgress == true {
		return i.parameters.ValidationInitPercentageThreshold.Int()
	}

	return i.parameters.ValidationPercentageThreshold.Int()
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
		connectorsToKeep       []string
		esIndicesToKeep        []string
		topicsToKeep           []string
		replicationSlotsToKeep []string
	)

	resourceNamePrefix := i.parameters.ResourceNamePrefix.String()

	if i.Instance.GetState() != xjoin.STATE_REMOVED && i.Instance.Status.PipelineVersion != "" {
		connectorsToKeep = append(connectorsToKeep, i.Kafka.DebeziumConnectorName(i.Instance.Status.PipelineVersion))
		connectorsToKeep = append(connectorsToKeep, i.Kafka.ESConnectorName(i.Instance.Status.PipelineVersion))
		esIndicesToKeep = append(esIndicesToKeep, i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion))
		topicsToKeep = append(topicsToKeep, i.Kafka.TopicName(i.Instance.Status.PipelineVersion))
		replicationSlotsToKeep = append(
			replicationSlotsToKeep,
			database.ReplicationSlotName(resourceNamePrefix, i.Instance.Status.PipelineVersion))
	}

	currentIndices, err := i.ESClient.GetCurrentIndicesWithAlias(i.Instance.Status.ActiveAliasName)
	if err != nil {
		errors = append(errors, err)
	} else if currentIndices != nil && i.Instance.GetState() != xjoin.STATE_REMOVED {
		version := currentIndices[0][len("xjoin.inventory.hosts"):len(currentIndices[0])]
		connectorsToKeep = append(connectorsToKeep, i.Kafka.DebeziumConnectorName(version))
		connectorsToKeep = append(connectorsToKeep, i.Kafka.ESConnectorName(version))
		esIndicesToKeep = append(esIndicesToKeep, currentIndices...)
		topicsToKeep = append(topicsToKeep, i.Kafka.TopicName(version))
		replicationSlotsToKeep = append(
			replicationSlotsToKeep, database.ReplicationSlotName(resourceNamePrefix, version))
	}

	//delete stale Kafka Connectors
	connectors, err := i.Kafka.ListConnectors()
	if err != nil {
		errors = append(errors, err)
	} else {
		for _, connector := range connectors.Items {
			if !utils.ContainsString(connectorsToKeep, connector.GetName()) {
				i.Log.Info("Removing stale connector", "connector", connector.GetName())
				if err = i.Kafka.DeleteConnector(connector.GetName()); err != nil {
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
			if !utils.ContainsString(topicsToKeep, topic) && strings.Index(topic, resourceNamePrefix) == 0 {
				i.Log.Info("Removing stale topic", "topic", topic)
				if err = i.Kafka.DeleteTopic(topic); err != nil {
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
		for _, slot := range slots {
			if !utils.ContainsString(replicationSlotsToKeep, slot) {
				i.Log.Info("Removing stale replication slot", "slot", slot)
				if err = i.InventoryDb.RemoveReplicationSlot(slot); err != nil {
					errors = append(errors, err)
				}
			}
		}
	}

	return
}

func (i *ReconcileIteration) recreateAliasIfNeeded() (bool, error) {
	currentIndices, err := i.ESClient.GetCurrentIndicesWithAlias(i.Instance.Status.ActiveAliasName)
	if err != nil {
		return false, err
	}

	validIndex := i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion)
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
	indices, err := i.ESClient.GetCurrentIndicesWithAlias(i.Instance.Status.ActiveAliasName)

	if err != nil {
		return fmt.Errorf("Failed to determine active index %w", err)
	}

	if indices != nil {
		if len(indices) == 1 && indices[0] == i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion) {
			return nil // index is already active, nothing to do
		}

		// no need to close this as that's done in ReconcileIteration.Close()
		i.InventoryDb = database.NewDatabase(database.DBParams{
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
		latestCount, err := i.ESClient.CountIndex(i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion))
		if err != nil {
			return fmt.Errorf("failed to get host count from latest index %w", err)
		}

		if utils.Abs(hbiHostCount-latestCount) > utils.Abs(hbiHostCount-activeCount) {
			return nil // the active table is healthier; do not update anything
		}
	}

	if err = i.ESClient.UpdateAliasByFullIndexName(
		i.parameters.ResourceNamePrefix.String(),
		i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion)); err != nil {
		return err
	}

	return nil
}

func (i *ReconcileIteration) checkForDeviation() (problem error, err error) {
	//Configmap/secrets
	if i.Instance.Status.XJoinConfigVersion != i.parameters.ConfigMapVersion.String() {
		return fmt.Errorf("configMap changed. New version is %s",
			i.parameters.ConfigMapVersion.String()), nil
	}

	if i.Instance.Status.ElasticSearchSecretVersion != i.parameters.ElasticSearchSecretVersion.String() {
		return fmt.Errorf("elasticsesarch secret changed. New version is %s",
			i.parameters.ElasticSearchSecretVersion.String()), nil
	}

	if i.Instance.Status.HBIDBSecretVersion != i.parameters.HBIDBSecretVersion.String() {
		return fmt.Errorf("hbidbsecret changed. New version is %s",
			i.parameters.HBIDBSecretVersion.String()), nil
	}

	//ES Index
	indexExists, err := i.ESClient.IndexExists(i.Instance.Status.PipelineVersion)

	if err != nil {
		return nil, err
	} else if indexExists == false {
		return fmt.Errorf(
			"elasticsearch index %s not found",
			i.ESClient.ESIndexName(i.Instance.Status.PipelineVersion)), nil
	}

	//Connectors
	esConnectorName := i.Kafka.ESConnectorName(i.Instance.Status.PipelineVersion)
	problem, err = i.checkConnectorDeviation(esConnectorName, "es")
	if err != nil || problem != nil {
		return problem, err
	}

	debeziumConnectorName := i.Kafka.DebeziumConnectorName(i.Instance.Status.PipelineVersion)
	problem, err = i.checkConnectorDeviation(debeziumConnectorName, "debezium")
	if err != nil || problem != nil {
		return problem, err
	}
	return nil, nil
}

func (i *ReconcileIteration) checkConnectorDeviation(connectorName string, connectorType string) (problem error, err error) {
	connector, err := i.Kafka.GetConnector(connectorName)
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
		return i.Kafka.CreateESConnector(i.Instance.Status.PipelineVersion, true)
	} else if conType == "debezium" {
		return i.Kafka.CreateDebeziumConnector(i.Instance.Status.PipelineVersion, true)
	} else {
		return nil, errors.New("invalid param. Must be one of [es, debezium]")
	}
}
