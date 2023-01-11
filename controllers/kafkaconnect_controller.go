package controllers

import (
	"context"
	"time"

	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type KafkaConnectReconciler struct {
	XJoinPipelineReconciler
	kafka           kafka.Kafka
	kafkaConnectors kafka.Connectors
	parameters      config.Parameters
	log             logger.Log
	instance        *xjoin.XJoinPipeline
}

func (r *KafkaConnectReconciler) Setup(reqLogger logger.Log, request ctrl.Request, ctx context.Context) error {
	instance, err := k8sUtils.FetchXJoinPipeline(r.Client, request.NamespacedName, ctx)
	if err != nil {
		if k8errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return nil
		}
		// Error reading the object - requeue the request.
		return err
	}
	r.instance = instance

	xjoinConfig, err := config.NewConfig(instance, r.Client, ctx)
	if xjoinConfig != nil {
		r.parameters = xjoinConfig.Parameters
	}
	if err != nil {
		return err
	}

	r.kafka = kafka.Kafka{
		Namespace:     request.Namespace,
		Client:        r.Client,
		Parameters:    r.parameters,
		ParametersMap: xjoinConfig.ParametersMap,
		Recorder:      r.Recorder,
		Test:          r.Test,
		GenericKafka: kafka.GenericKafka{
			Context:          ctx,
			Client:           r.Client,
			ConnectNamespace: r.parameters.ConnectClusterNamespace.String(),
			ConnectCluster:   r.parameters.ConnectCluster.String(),
			KafkaNamespace:   r.parameters.KafkaClusterNamespace.String(),
			KafkaCluster:     r.parameters.KafkaCluster.String(),
		},
	}

	kafkaTopics := &kafka.StrimziTopics{
		TopicParameters: kafka.TopicParameters{
			Replicas:           r.parameters.KafkaTopicReplicas.Int(),
			Partitions:         r.parameters.KafkaTopicPartitions.Int(),
			CleanupPolicy:      r.parameters.KafkaTopicCleanupPolicy.String(),
			MinCompactionLagMS: r.parameters.KafkaTopicMinCompactionLagMS.String(),
			RetentionBytes:     r.parameters.KafkaTopicRetentionBytes.String(),
			RetentionMS:        r.parameters.KafkaTopicRetentionMS.String(),
			MessageBytes:       r.parameters.KafkaTopicMessageBytes.String(),
			CreationTimeout:    r.parameters.KafkaTopicCreationTimeout.Int(),
		},
		KafkaClusterNamespace: r.parameters.KafkaClusterNamespace.String(),
		KafkaCluster:          r.parameters.KafkaCluster.String(),
		Client:                r.Client,
		Test:                  r.Test,
		Context:               ctx,
		ResourceNamePrefix:    r.parameters.ResourceNamePrefix.String(),
	}

	r.kafkaConnectors = &kafka.StrimziConnectors{
		Kafka:  r.kafka,
		Topics: kafkaTopics,
	}

	r.log = reqLogger

	return nil
}

// Reconcile
// This reconciles Kafka Connect is running and each active connector is running
func (r *KafkaConnectReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := logger.NewLogger("controller_kafkaconnect", "Pipeline", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling Kafka Connect")

	err := r.Setup(reqLogger, request, ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	// skip kafka connect reconciliation in ephemeral namespaces
	if r.parameters.Ephemeral.Bool() {
		reqLogger.Info("Skipping kafkaconnect reconciliation")
		return reconcile.Result{}, nil
	}

	err = r.reconcileKafkaConnect()
	if err != nil {
		reqLogger.Error(err, "Error while reconciling Kafka Connect")
	}

	if r.instance.Status.ActiveESConnectorName != "" {
		err = r.reconcileKafkaConnector(r.instance.Status.ActiveESConnectorName)
		if err != nil {
			reqLogger.Error(err, "Error while reconciling ES connector")
		}
	}

	if r.instance.Status.ActiveDebeziumConnectorName != "" {
		err = r.reconcileKafkaConnector(r.instance.Status.ActiveDebeziumConnectorName)
		if err != nil {
			reqLogger.Error(err, "Error while reconciling Debezium connector")
		}
	}

	if err != nil {
		return reconcile.Result{}, err
	} else {
		return reconcile.Result{
			RequeueAfter: time.Duration(r.parameters.KafkaConnectReconcileIntervalSeconds.Int()) * time.Second}, nil
	}
}

func (r *KafkaConnectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-kafkaconnect").
		For(&xjoin.XJoinPipeline{}).
		WithEventFilter(eventFilterPredicate()).
		WithLogger(mgr.GetLogger()).
		WithOptions(controller.Options{
			Log:         mgr.GetLogger(),
			RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, 1*time.Minute),
		}).
		Complete(r)
}

func NewKafkaConnectReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
	recorder record.EventRecorder,
	namespace string,
	isTest bool) *KafkaConnectReconciler {

	return &KafkaConnectReconciler{
		XJoinPipelineReconciler: *NewXJoinReconciler(client, scheme, log, recorder, namespace, isTest),
	}
}

func (r *KafkaConnectReconciler) reconcileKafkaConnector(connectorName string) error {
	isFailed, err := r.kafkaConnectors.IsFailed(connectorName)
	if err != nil {
		return err
	}

	if isFailed {
		r.log.Warn("Connector is failed, restarting it.", "connector", connectorName)
		err = r.kafkaConnectors.RestartConnector(connectorName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *KafkaConnectReconciler) reconcileKafkaConnect() error {
	connectIsUp, err := r.kafkaConnectors.CheckIfConnectIsResponding()
	if err != nil {
		return err
	}

	if !connectIsUp {
		err = r.kafkaConnectors.RestartConnect()
		if err != nil {
			return err
		}
	}

	return nil
}
