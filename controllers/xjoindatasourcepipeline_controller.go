package controllers

import (
	"context"
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/components"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	. "github.com/redhatinsights/xjoin-operator/controllers/datasource"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const xjoindatasourcepipelineFinalizer = "finalizer.xjoin.datasourcepipeline.cloud.redhat.com"

type XJoinDataSourcePipelineReconciler struct {
	Client    client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Namespace string
	Test      bool
}

func NewXJoinDataSourcePipelineReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
	recorder record.EventRecorder,
	namespace string,
	isTest bool) *XJoinDataSourcePipelineReconciler {

	return &XJoinDataSourcePipelineReconciler{
		Client:    client,
		Log:       log,
		Scheme:    scheme,
		Recorder:  recorder,
		Namespace: namespace,
		Test:      isTest,
	}
}

func (r *XJoinDataSourcePipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	logConstructor := func(r *reconcile.Request) logr.Logger {
		return mgr.GetLogger()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-datasourcepipeline-controller").
		For(&xjoin.XJoinDataSourcePipeline{}).
		WithLogConstructor(logConstructor).
		WithOptions(controller.Options{
			LogConstructor: logConstructor,
			RateLimiter:    workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, 1*time.Minute),
		}).
		Complete(r)
}

// +kubebuilder:rbac:groups=xjoin.cloud.redhat.com,resources=xjoindatasourcepipelines;xjoindatasourcepipelines/status;xjoindatasourcepipelines/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkaconnectors;kafkaconnectors/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkatopics;kafkatopics/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkaconnects;kafkas,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps;pods;deployments,verbs=get;list;watch

func (r *XJoinDataSourcePipelineReconciler) Reconcile(ctx context.Context, request ctrl.Request) (result ctrl.Result, err error) {
	reqLogger := xjoinlogger.NewLogger("controller_xjoindatasourcepipeline", "DataSourcePipeline", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling XJoinDataSourcePipeline")

	instance, err := k8sUtils.FetchXJoinDataSourcePipeline(r.Client, request.NamespacedName, ctx)
	if err != nil {
		if k8errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return result, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	p := parameters.BuildDataSourceParameters()

	configManager, err := config.NewManager(config.ManagerOptions{
		Client:            r.Client,
		Parameters:        p,
		ConfigMapNames:    []string{"xjoin-generic"},
		SecretNames:       nil,
		ResourceNamespace: instance.Namespace,
		OperatorNamespace: r.Namespace,
		Spec:              instance.Spec,
		Context:           ctx,
		Log:               reqLogger,
		Ephemeral:         instance.Spec.Ephemeral,
	})
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}
	err = configManager.Parse()
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	if p.Pause.Bool() {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	i := XJoinDataSourcePipelineIteration{
		Parameters: *p,
		Iteration: common.Iteration{
			Context:          ctx,
			Instance:         instance,
			OriginalInstance: instance.DeepCopy(),
			Client:           r.Client,
			Log:              reqLogger,
		},
	}

	if err = i.AddFinalizer(xjoindatasourcepipelineFinalizer); err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	kafkaClient := kafka.GenericKafka{
		Context:          ctx,
		ConnectNamespace: p.ConnectClusterNamespace.String(),
		ConnectCluster:   p.ConnectCluster.String(),
		KafkaNamespace:   p.KafkaClusterNamespace.String(),
		KafkaCluster:     p.KafkaCluster.String(),
		Client:           i.Client,
		Test:             r.Test,
		Log:              reqLogger,
	}

	registry := schemaregistry.NewSchemaRegistryConfluentClient(
		schemaregistry.ConnectionParams{
			Protocol: p.SchemaRegistryProtocol.String(),
			Hostname: p.SchemaRegistryHost.String(),
			Port:     p.SchemaRegistryPort.String(),
		})

	registry.Init()

	e := events.NewEvents(r.Recorder, instance, reqLogger)
	componentManager := components.NewComponentManager(
		common.DataSourcePipelineGVK.Kind, instance.Spec.Name, p.Version.String(), e, reqLogger)
	componentManager.AddComponent(components.NewAvroSchema(components.AvroSchemaParameters{
		Schema:      p.AvroSchema.String(),
		Registry:    registry,
		KafkaClient: kafkaClient,
	}))

	kafkaTopics := kafka.StrimziTopics{
		TopicParameters: kafka.TopicParameters{
			Replicas:           p.KafkaTopicReplicas.Int(),
			Partitions:         p.KafkaTopicPartitions.Int(),
			CleanupPolicy:      p.KafkaTopicCleanupPolicy.String(),
			MinCompactionLagMS: p.KafkaTopicMinCompactionLagMS.String(),
			RetentionBytes:     p.KafkaTopicRetentionBytes.String(),
			RetentionMS:        p.KafkaTopicRetentionMS.String(),
			MessageBytes:       p.KafkaTopicMessageBytes.String(),
			CreationTimeout:    p.KafkaTopicCreationTimeout.Int(),
		},
		KafkaClusterNamespace: p.KafkaClusterNamespace.String(),
		KafkaCluster:          p.KafkaCluster.String(),
		Client:                i.Client,
		Test:                  r.Test,
		Context:               ctx,
		//ResourceNamePrefix:  this is not needed for generic topics
	}
	componentManager.AddComponent(&components.KafkaTopic{
		TopicParameters: kafka.TopicParameters{
			Replicas:           p.KafkaTopicReplicas.Int(),
			Partitions:         p.KafkaTopicPartitions.Int(),
			CleanupPolicy:      p.KafkaTopicCleanupPolicy.String(),
			MinCompactionLagMS: p.KafkaTopicMinCompactionLagMS.String(),
			RetentionBytes:     p.KafkaTopicRetentionBytes.String(),
			RetentionMS:        p.KafkaTopicRetentionMS.String(),
			MessageBytes:       p.KafkaTopicMessageBytes.String(),
			CreationTimeout:    p.KafkaTopicCreationTimeout.Int(),
		},
		KafkaTopics: kafkaTopics,
		KafkaClient: kafkaClient,
	})

	componentManager.AddComponent(&components.DebeziumConnector{
		TemplateParameters: config.ParametersToMap(*p),
		KafkaClient:        kafkaClient,
		Template:           p.DebeziumConnectorTemplate.String(),
		Namespace:          instance.Namespace,
	})

	db := database.NewDatabase(database.DBParams{
		User:        p.DatabaseUsername.String(),
		Password:    p.DatabasePassword.String(),
		Host:        p.DatabaseHostname.String(),
		Name:        p.DatabaseName.String(),
		Port:        p.DatabasePort.String(),
		SSLMode:     p.DatabaseSSLMode.String(),
		SSLRootCert: p.DatabaseSSLRootCert.String(),
		IsTest:      r.Test,
	})
	err = db.Connect()
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}
	defer db.Close()

	componentManager.AddComponent(&components.ReplicationSlot{
		Namespace: instance.Namespace,
		Database:  db,
	})

	if instance.GetDeletionTimestamp() != nil {
		reqLogger.Info("Starting finalizer")
		errs := componentManager.DeleteAll()
		if len(errs) > 0 {
			for _, componentErr := range errs {
				reqLogger.Error(componentErr, "error deleting component during finalizer")
			}
			return reconcile.Result{}, errors.New("error deleting components during finalizer")
		}

		controllerutil.RemoveFinalizer(instance, xjoindatasourcepipelineFinalizer)
		ctx, cancel := utils.DefaultContext()
		defer cancel()
		err = r.Client.Update(ctx, instance)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, 0)
		}

		reqLogger.Info("Successfully finalized")
		return reconcile.Result{}, nil
	}

	err = componentManager.CreateAll()
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	err = componentManager.Reconcile()
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	problems, err := componentManager.CheckForDeviations()
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	if len(problems) > 0 {
		i.GetInstance().SetCondition(metav1.Condition{
			Type:   common.ValidConditionType,
			Status: metav1.ConditionFalse,
			Reason: common.DeviationReason,
		})
		i.GetInstance().Status.ValidationResponse.Result = validation.ValidationInvalid
		i.GetInstance().Status.ValidationResponse.Reason = "Deviation found"
		var messages []string
		for _, problem := range problems {
			messages = append(messages, problem.Error())
		}
		i.GetInstance().Status.ValidationResponse.Message = strings.Join(messages, ", ")
		reqLogger.Warn("Deviation found", "problems", problems)
	}

	i.UpdateCondition()
	return i.UpdateStatusAndRequeue(time.Second * 30)
}
