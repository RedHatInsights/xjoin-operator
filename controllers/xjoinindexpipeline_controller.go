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
	"k8s.io/utils/strings/slices"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
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

		// trigger Reconcile if DataSource changes
		Watches(
			&source.Kind{Type: &xjoin.XJoinDataSourcePipeline{}},
			handler.EnqueueRequestsFromMapFunc(func(dataSourcePipeline client.Object) (requests []reconcile.Request) {
				ctx, cancel := utils.DefaultContext()
				defer cancel()

				indexPipelines, err := k8sUtils.FetchXJoinIndexPipelines(r.Client, ctx)
				if err != nil {
					r.Log.Error(err, "Failed to fetch IndexPipelines in Watch")
					return requests
				}

				for _, indexPipeline := range indexPipelines.Items {
					if slices.Contains(indexPipeline.GetDataSourcePipelineNames(), dataSourcePipeline.GetName()) {
						requests = append(requests, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Namespace: indexPipeline.GetNamespace(),
								Name:      indexPipeline.GetName(),
							},
						})
					}
				}

				return requests
			}),
		).
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
			reqLogger.Warn("XJoinIndex not found", "XJoinIndex", request.Name)
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

	var elasticsearchKubernetesSecretName string
	if instance.Spec.Ephemeral {
		elasticsearchKubernetesSecretName = "xjoin-elasticsearch-es-elastic-user"
	} else {
		elasticsearchKubernetesSecretName = "xjoin-elasticsearch"
	}

	configManager, err := config.NewManager(config.ManagerOptions{
		Client:         r.Client,
		Parameters:     p,
		ConfigMapNames: []string{"xjoin-generic"},
		SecretNames: []config.SecretNames{{
			KubernetesName: elasticsearchKubernetesSecretName,
			ManagerName:    parameters.ElasticsearchSecret,
		}},
		ResourceNamespace: instance.Namespace,
		OperatorNamespace: r.Namespace,
		Spec:              instance.Spec,
		Context:           ctx,
		Ephemeral:         instance.Spec.Ephemeral,
		Log:               reqLogger,
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
	registryRestClient := schemaregistry.NewSchemaRegistryRestClient(schemaRegistryConnectionParams, r.Namespace)

	indexAvroSchemaParser := avro.IndexAvroSchemaParser{
		AvroSchema:      p.AvroSchema.String(),
		Client:          i.Client,
		Context:         i.Context,
		Namespace:       i.Instance.GetNamespace(),
		Log:             i.Log,
		SchemaRegistry:  confluentClient,
		SchemaNamespace: i.Instance.GetName(),
		Active:          i.GetInstance().Status.Active,
	}
	indexAvroSchema, err := indexAvroSchemaParser.Parse()
	if err != nil {
		return result, errors.Wrap(err, 0)
	}

	componentManager := components.NewComponentManager(common.IndexPipelineGVK.Kind, instance.Spec.Name, p.Version.String())

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
		Active:   i.GetInstance().Status.Active,
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
		Ephemeral:              i.GetInstance().Spec.Ephemeral,
	})

	for _, customSubgraphImage := range instance.Spec.CustomSubgraphImages {
		customSubgraphGraphQLSchemaComponent := components.NewGraphQLSchema(components.GraphQLSchemaParameters{
			Registry: registryRestClient,
			Suffix:   customSubgraphImage.Name,
			Active:   i.GetInstance().Status.Active,
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

	err = componentManager.Reconcile()
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	//check each datasource is valid
	dataSources := make(map[string]string)
	allDataSourcesValid := true
	for _, ref := range indexAvroSchema.References {
		//get each datasource name and resource version
		datasourceName := strings.Split(ref.Name, ".Value")[0]

		datasourcePipelineVersion := strings.Split(ref.Subject, "xjoindatasourcepipeline.")[1]
		datasourcePipelineVersion = strings.Split(datasourcePipelineVersion, ".")[1]
		datasourcePipelineVersion = strings.Split(datasourcePipelineVersion, "-value")[0]

		dataSources[datasourceName] = datasourcePipelineVersion

		//GET each datasourcePipeline
		dataSourcePipelineName := types.NamespacedName{
			Namespace: instance.GetNamespace(),
			Name:      datasourceName + "." + datasourcePipelineVersion,
		}
		dataSourcePipeline, err := k8sUtils.FetchXJoinDataSourcePipeline(r.Client, dataSourcePipelineName, ctx)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, 0)
		}

		if dataSourcePipeline.Status.ValidationResponse.Result == Invalid {
			allDataSourcesValid = false
		}
	}

	if allDataSourcesValid {
		instance.Status.ValidationResponse.Result = Valid
	} else {
		instance.Status.ValidationResponse.Result = Invalid
	}

	if !reflect.DeepEqual(instance.Status.DataSources, dataSources) {
		instance.Status.DataSources = dataSources
	}

	//deviation check
	problems, err := componentManager.CheckForDeviations()
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, 0)
	}

	if len(problems) > 0 {
		i.GetInstance().Status.ValidationResponse.Result = Invalid
		i.GetInstance().Status.ValidationResponse.Reason = "Deviation found"
		var messages []string
		for _, problem := range problems {
			messages = append(messages, problem.Error())
		}
		i.GetInstance().Status.ValidationResponse.Message = strings.Join(messages, ", ")
		reqLogger.Warn("Deviation found", "problems", problems)
	}

	i.Instance = instance
	return i.UpdateStatusAndRequeue(time.Second * 30)
}
