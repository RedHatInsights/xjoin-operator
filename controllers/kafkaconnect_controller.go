package controllers

import (
	"context"
	"errors"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/metrics"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	corev1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"strconv"
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

func (r *KafkaConnectReconciler) Setup(reqLogger logger.Log, request ctrl.Request) error {
	instance, err := utils.FetchXJoinPipeline(r.Client, request.NamespacedName)
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

	xjoinConfig, err := config.NewConfig(instance, r.Client)
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
func (r *KafkaConnectReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	reqLogger := logger.NewLogger("controller_validation", "Pipeline", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling Kafka Connect")

	err := r.Setup(reqLogger, request)
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
		metrics.ConnectRestarted()
		r.log.Warn("Restarting Kafka Connect")

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()

		//get current pod template hash
		pods := &corev1.PodList{}

		labels := client.MatchingLabels{}
		labels["app.kubernetes.io/part-of"] = "strimzi-" + r.parameters.ConnectCluster.String()
		err = r.Client.List(ctx, pods, labels)
		if err != nil {
			return err
		}

		if len(pods.Items) < 1 {
			return errors.New("no Kafka Connect instance found")
		}

		podLabels := pods.Items[0].GetLabels()
		currentHash := podLabels["pod-template-hash"]

		//retrieve connect object
		var connectGVK = schema.GroupVersionKind{
			Group:   "kafka.strimzi.io",
			Kind:    "KafkaConnect",
			Version: "v1beta1",
		}

		connect := &unstructured.Unstructured{}
		connect.SetGroupVersionKind(connectGVK)
		err = r.Client.Get(
			ctx,
			client.ObjectKey{Name: r.parameters.ConnectCluster.String(), Namespace: r.parameters.ConnectClusterNamespace.String()},
			connect)
		if err != nil {
			return err
		}

		//update connect object label to trigger redeploy
		obj := connect.Object
		metadata := make(map[string]interface{})
		if obj["metadata"] != nil {
			metadata = obj["metadata"].(map[string]interface{})
		}

		connectLabels := make(map[string]interface{})
		if metadata["labels"] != nil {
			connectLabels = metadata["labels"].(map[string]interface{})
		}
		now := strconv.Itoa(int(time.Now().Unix()))
		connectLabels["redeploy"] = now
		metadata["labels"] = connectLabels
		obj["metadata"] = metadata

		err = r.Client.Update(ctx, connect)
		if err != nil {
			return err
		}

		r.log.Info("Waiting for Kafka Connect to be ready...")
		//wait for deployment to complete by checking each pods status
		err = wait.PollImmediate(time.Second*time.Duration(10), time.Duration(1800)*time.Second, func() (bool, error) {
			pods := &corev1.PodList{}

			ctx, cancel = context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()

			labels := client.MatchingLabels{}
			labels["app.kubernetes.io/part-of"] = "strimzi-" + r.parameters.ConnectCluster.String()
			err = r.Client.List(ctx, pods, labels)

			//only check pods for new deployment
			var newPods []corev1.Pod

			for _, pod := range pods.Items {
				podLabels := pod.GetLabels()
				if podLabels["pod-template-hash"] != currentHash {
					newPods = append(newPods, pod)
				}
			}

			if len(newPods) == 0 {
				return false, nil
			}

			//check the status of each pod for the new deployment
			for _, pod := range pods.Items {
				for _, condition := range pod.Status.Conditions {
					if condition.Type == "Ready" {
						if condition.Status != "True" {
							return false, nil
						}
					}
				}
			}

			return true, nil
		})

		if err != nil {
			return err
		}

		r.log.Info("Kafka Connect successfully restarted")
	}

	return nil
}
