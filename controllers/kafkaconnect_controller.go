package controllers

import (
	"context"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type KafkaConnectReconciler struct {
	XJoinPipelineReconciler
	kafka      kafka.Kafka
	parameters config.Parameters
	log        logger.Log
	instance   *xjoin.XJoinPipeline
}

func (r *KafkaConnectReconciler) Setup(reqLogger logger.Log, request ctrl.Request, ctx context.Context) error {
	instance, err := utils.FetchXJoinPipeline(r.Client, request.NamespacedName, ctx)
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
	}

	r.log = reqLogger

	return nil
}

// Reconcile
// This reconciles Kafka Connect is running and each active connector is running
func (r *KafkaConnectReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := logger.NewLogger("controller_validation", "Pipeline", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling Kafka Connect")

	err := r.Setup(reqLogger, request, ctx)
	if err != nil {
		return reconcile.Result{}, err
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
	isFailed, err := r.kafka.IsFailed(connectorName)
	if err != nil {
		return err
	}

	if isFailed {
		r.log.Warn("Connector is failed, restarting it.", "connector", connectorName)
		err = r.kafka.RestartConnector(connectorName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *KafkaConnectReconciler) reconcileKafkaConnect() error {
	connectIsUp, err := r.kafka.CheckIfConnectIsResponding()
	if err != nil {
		return err
	}

	if !connectIsUp {
		err = r.kafka.RestartConnect()
		if err != nil {
			return err
		}
	}

	return nil
}
