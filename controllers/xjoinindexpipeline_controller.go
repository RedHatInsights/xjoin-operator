package controllers

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/avro"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/components"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	. "github.com/redhatinsights/xjoin-operator/controllers/index"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"time"
)

const xjoinindexpipelineFinalizer = "finalizer.xjoin.indexpipeline.cloud.redhat.com"

type XJoinIndexPipelineReconciler struct {
	Client    client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Namespace string
	Test      bool
}

func NewXJoinIndexPipelineReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
	recorder record.EventRecorder,
	namespace string,
	isTest bool) *XJoinIndexPipelineReconciler {

	return &XJoinIndexPipelineReconciler{
		Client:    client,
		Log:       log,
		Scheme:    scheme,
		Recorder:  recorder,
		Namespace: namespace,
		Test:      isTest,
	}
}

func (r *XJoinIndexPipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	logConstructor := func(r *reconcile.Request) logr.Logger {
		return mgr.GetLogger()
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-indexpipeline-controller").
		For(&xjoin.XJoinIndexPipeline{}).
		WithLogConstructor(logConstructor).
		WithOptions(controller.Options{
			LogConstructor: logConstructor,
			RateLimiter:    workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, 1*time.Minute),
		}).
		Complete(r)
}

// +kubebuilder:rbac:groups=xjoin.cloud.redhat.com,resources=xjoinindexpipelines;xjoinindexpipelines/status;xjoinindexpipelines/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkaconnectors;kafkaconnectors/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkatopics;kafkatopics/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.strimzi.io,resources=kafkaconnects;kafkas,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps;pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services;events,verbs=get;list;watch;create;delete;update
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;delete;update

func (r *XJoinIndexPipelineReconciler) Reconcile(ctx context.Context, request ctrl.Request) (result ctrl.Result, err error) {
	reqLogger := xjoinlogger.NewLogger("controller_xjoinindexpipeline", "IndexPipeline", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling XJoinIndexPipeline")

	instance, err := k8sUtils.FetchXJoinIndexPipeline(r.Client, request.NamespacedName, ctx)
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

	if len(instance.OwnerReferences) < 1 {
		return reconcile.Result{}, errors.Wrap(errors.New(
			"Missing OwnerReference from xjoinindexpipeline. "+
				"XJoinIndexPipeline resources must be managed by an XJoinIndex. "+
				"XJoinIndexPipeline cannot be created individually."), 0)
	}

	p := parameters.BuildIndexParameters()

	configManager, err := config.NewManager(config.ManagerOptions{
		Client:         r.Client,
		Parameters:     p,
		ConfigMapNames: []string{"xjoin-generic"},
		SecretNames:    []string{"xjoin-elasticsearch"},
		Namespace:      instance.Namespace,
		Spec:           instance.Spec,
		Context:        ctx,
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

	i := XJoinIndexPipelineIteration{
		Parameters: *p,
		Iteration: common.Iteration{
			Context:          ctx,
			Instance:         instance,
			OriginalInstance: instance.DeepCopy(),
			Client:           r.Client,
			Log:              reqLogger,
		},
	}

	if err = i.AddFinalizer(xjoinindexpipelineFinalizer); err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	parametersMap := config.ParametersToMap(*p)

	kafkaClient := kafka.GenericKafka{
		Context:          ctx,
		ConnectNamespace: p.ConnectClusterNamespace.String(),
		ConnectCluster:   p.ConnectCluster.String(),
		KafkaNamespace:   p.KafkaClusterNamespace.String(),
		KafkaCluster:     p.KafkaCluster.String(),
		Client:           i.Client,
		Test:             r.Test,
	}

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
		Client:                r.Client,
		Test:                  r.Test,
		Context:               ctx,
	}

	kafkaTopic := &components.KafkaTopic{
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
	}

	elasticSearchConnection := elasticsearch.GenericElasticSearchParameters{
		Url:        p.ElasticSearchURL.String(),
		Username:   p.ElasticSearchUsername.String(),
		Password:   p.ElasticSearchPassword.String(),
		Parameters: parametersMap,
		Context:    i.Context,
	}
	genericElasticsearch, err := elasticsearch.NewGenericElasticsearch(elasticSearchConnection)
	if err != nil {
		return result, errors.Wrap(err, 0)
	}

	schemaRegistryConnectionParams := schemaregistry.ConnectionParams{
		Protocol: p.SchemaRegistryProtocol.String(),
		Hostname: p.SchemaRegistryHost.String(),
		Port:     p.SchemaRegistryPort.String(),
	}
	confluentClient := schemaregistry.NewSchemaRegistryConfluentClient(schemaRegistryConnectionParams)
	confluentClient.Init()
	registryRestClient := schemaregistry.NewSchemaRegistryRestClient(schemaRegistryConnectionParams)

	indexAvroSchemaParser := avro.IndexAvroSchemaParser{
		AvroSchema:      p.AvroSchema.String(),
		Client:          i.Client,
		Context:         i.Context,
		Namespace:       i.Instance.GetNamespace(),
		Log:             i.Log,
		SchemaRegistry:  confluentClient,
		SchemaNamespace: i.Instance.GetName(),
	}
	indexAvroSchema, err := indexAvroSchemaParser.Parse()
	if err != nil {
		return result, errors.Wrap(err, 0)
	}

	componentManager := components.NewComponentManager(common.IndexPipelineGVK.Kind+"."+instance.Spec.Name, p.Version.String())

	if indexAvroSchema.JSONFields != nil {
		componentManager.AddComponent(&components.ElasticsearchPipeline{
			GenericElasticsearch: *genericElasticsearch,
			JsonFields:           indexAvroSchema.JSONFields,
		})
	}

	elasticSearchIndexComponent := &components.ElasticsearchIndex{
		GenericElasticsearch: *genericElasticsearch,
		Template:             p.ElasticSearchIndexTemplate.String(),
		Properties:           indexAvroSchema.ESProperties,
		WithPipeline:         indexAvroSchema.JSONFields != nil,
	}
	componentManager.AddComponent(elasticSearchIndexComponent)
	componentManager.AddComponent(kafkaTopic)
	componentManager.AddComponent(&components.ElasticsearchConnector{
		Template:           p.ElasticSearchConnectorTemplate.String(),
		KafkaClient:        kafkaClient,
		TemplateParameters: parametersMap,
		Topic:              kafkaTopic.Name(),
	})
	componentManager.AddComponent(components.NewAvroSchema(components.AvroSchemaParameters{
		Schema:   indexAvroSchema.AvroSchemaString,
		Registry: confluentClient,
	}))
	graphqlSchemaComponent := components.NewGraphQLSchema(components.GraphQLSchemaParameters{
		Registry: registryRestClient,
	})
	componentManager.AddComponent(graphqlSchemaComponent)
	componentManager.AddComponent(&components.XJoinCore{
		Client:            i.Client,
		Context:           i.Context,
		SourceTopics:      indexAvroSchema.SourceTopics,
		SinkTopic:         indexAvroSchemaParser.AvroSubjectToKafkaTopic(kafkaTopic.Name()),
		KafkaBootstrap:    p.KafkaBootstrapURL.String(),
		SchemaRegistryURL: p.SchemaRegistryProtocol.String() + "://" + p.SchemaRegistryHost.String() + ":" + p.SchemaRegistryPort.String(),
		Namespace:         i.Instance.GetNamespace(),
		Schema:            indexAvroSchema.AvroSchemaString,
	})
	componentManager.AddComponent(&components.XJoinAPISubGraph{
		Client:                i.Client,
		Context:               i.Context,
		Namespace:             i.Instance.GetNamespace(),
		AvroSchema:            indexAvroSchema.AvroSchemaString,
		Registry:              confluentClient,
		ElasticSearchURL:      p.ElasticSearchURL.String(),
		ElasticSearchUsername: p.ElasticSearchUsername.String(),
		ElasticSearchPassword: p.ElasticSearchPassword.String(),
		ElasticSearchIndex:    elasticSearchIndexComponent.Name(),
		Image:                 "quay.io/cloudservices/xjoin-api-subgraph:latest", //TODO
		GraphQLSchemaName:     graphqlSchemaComponent.Name(),
	})
	componentManager.AddComponent(&components.XJoinIndexValidator{
		Client:                 i.Client,
		Context:                i.Context,
		Namespace:              i.Instance.GetNamespace(),
		Schema:                 p.AvroSchema.String(),
		Pause:                  i.Parameters.Pause.Bool(),
		ParentInstance:         i.Instance,
		ElasticsearchIndexName: elasticSearchIndexComponent.Name(),
	})

	for _, customSubgraphImage := range instance.Spec.CustomSubgraphImages {
		customSubgraphGraphQLSchemaComponent := components.NewGraphQLSchema(components.GraphQLSchemaParameters{
			Registry: registryRestClient,
			Suffix:   customSubgraphImage.Name,
		})
		componentManager.AddComponent(customSubgraphGraphQLSchemaComponent)
		componentManager.AddComponent(&components.XJoinAPISubGraph{
			Client:                i.Client,
			Context:               i.Context,
			Namespace:             i.Instance.GetNamespace(),
			AvroSchema:            indexAvroSchema.AvroSchemaString,
			Registry:              confluentClient,
			ElasticSearchURL:      p.ElasticSearchURL.String(),
			ElasticSearchUsername: p.ElasticSearchUsername.String(),
			ElasticSearchPassword: p.ElasticSearchPassword.String(),
			ElasticSearchIndex:    elasticSearchIndexComponent.Name(),
			Image:                 customSubgraphImage.Image,
			Suffix:                customSubgraphImage.Name,
			GraphQLSchemaName:     customSubgraphGraphQLSchemaComponent.Name(),
		})
	}

	if instance.GetDeletionTimestamp() != nil {
		reqLogger.Info("Starting finalizer")
		err = componentManager.DeleteAll()
		if err != nil {
			reqLogger.Error(err, "error deleting components during finalizer")
			return
		}

		controllerutil.RemoveFinalizer(instance, xjoinindexpipelineFinalizer)
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

	problems, err := componentManager.CheckForDeviations()
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	if len(problems) > 0 {
		//TODO: set instance status to invalid, add problems to status
		reqLogger.Info("TODO: Set Instance status to invalid, add", "problems", len(problems))
	}

	//build list of datasources
	dataSources := make(map[string]string)
	for _, ref := range indexAvroSchema.References {
		//get each datasource name and resource version
		name := strings.Split(ref.Name, "xjoindatasourcepipeline.")[1]
		name = strings.Split(name, ".Value")[0]
		datasourceNamespacedName := types.NamespacedName{
			Name:      name,
			Namespace: i.Instance.GetNamespace(),
		}
		datasource, err := k8sUtils.FetchXJoinDataSource(i.Client, datasourceNamespacedName, ctx)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, 0)
		}
		dataSources[name] = datasource.ResourceVersion
	}

	//update parent status
	indexNamespacedName := types.NamespacedName{
		Name:      instance.OwnerReferences[0].Name,
		Namespace: i.Instance.GetNamespace(),
	}
	xjoinIndex, err := k8sUtils.FetchXJoinIndex(i.Client, indexNamespacedName, ctx)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	if !reflect.DeepEqual(xjoinIndex.Status.DataSources, dataSources) {
		xjoinIndex.Status.DataSources = dataSources

		if err := i.Client.Status().Update(ctx, xjoinIndex); err != nil {
			if k8errors.IsConflict(err) {
				i.Log.Error(err, "Status conflict")
				return reconcile.Result{}, err
			}

			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{RequeueAfter: time.Second * 30}, nil
}
