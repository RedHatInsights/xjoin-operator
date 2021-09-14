package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/components"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strconv"
	"time"
)

const xjoindatasourceFinalizer = "finalizer.xjoin.datasource.cloud.redhat.com"

var dataSourcePipelineGVK = schema.GroupVersionKind{
	Group:   "xjoin.cloud.redhat.com",
	Kind:    "XJoinDataSourcePipeline",
	Version: "v1alpha1",
}

type XJoinDataSourceReconciler struct {
	Client    client.Client
	Log       logr.Logger
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Namespace string
	Test      bool
}

func NewXJoinDataSourceReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	log logr.Logger,
	recorder record.EventRecorder,
	namespace string,
	isTest bool) *XJoinDataSourceReconciler {

	return &XJoinDataSourceReconciler{
		Client:    client,
		Log:       log,
		Scheme:    scheme,
		Recorder:  recorder,
		Namespace: namespace,
		Test:      isTest,
	}
}

func (r *XJoinDataSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("xjoin-datasource-controller").
		For(&xjoin.XJoinDataSource{}).
		Complete(r)
}

func (r *XJoinDataSourceReconciler) finalize(
	instance *xjoin.XJoinDataSource, componentManager components.ComponentManager) (result ctrl.Result, err error) {

	r.Log.Info("Starting finalizer")
	err = componentManager.DeleteAll()
	if err != nil {
		return
	}

	controllerutil.RemoveFinalizer(instance, xjoindatasourceFinalizer)
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = r.Client.Update(ctx, instance)
	if err != nil {
		return
	}

	r.Log.Info("Successfully finalized")
	return reconcile.Result{}, nil
}

// +kubebuilder:rbac:groups=xjoin.cloud.redhat.com,resources=xjoindatasources;xjoindatasources/status;xjoindatasources/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;pods,verbs=get;list;watch

func (r *XJoinDataSourceReconciler) Reconcile(ctx context.Context, request ctrl.Request) (result ctrl.Result, err error) {
	reqLogger := xjoinlogger.NewLogger("controller_xjoindatasource", "DataSource", request.Name, "Namespace", request.Namespace)
	reqLogger.Info("Reconciling XJoinDataSource")

	instance, err := utils.FetchXJoinDataSource(r.Client, request.NamespacedName, ctx)
	if err != nil {
		if k8errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return result, nil
		}
		// Error reading the object - requeue the request.
		return
	}

	p := parameters.BuildDataSourceParameters()

	configManager, err := config.NewManager(config.ManagerOptions{
		Client:         r.Client,
		Parameters:     p,
		ConfigMapNames: []string{"xjoin"},
		SecretNames:    nil,
		Namespace:      instance.Namespace,
		Spec:           instance.Spec,
		Context:        ctx,
	})
	if err != nil {
		return
	}
	err = configManager.Parse()
	if err != nil {
		return
	}

	if p.Pause.Bool() == true {
		return
	}

	i := XJoinDataSourceInstance{
		instance:   instance,
		client:     r.Client,
		log:        reqLogger,
		parameters: *p,
	}

	//TODO: version management
	version := instance.Status.ActiveVersion
	if version == "" {
		version = fmt.Sprintf("%s", strconv.FormatInt(time.Now().UnixNano(), 10))
	}

	componentManager := components.NewComponentManager(version)
	componentManager.AddComponent(components.NewDataSourcePipeline())

	if instance.GetDeletionTimestamp() != nil {
		return r.finalize(instance, componentManager)
	}

	err = componentManager.CreateAll()
	if err != nil {
		return
	}

	err = i.createDataSourcePipeline(instance.Name, version)
	if err != nil {
		return
	}

	return reconcile.Result{RequeueAfter: time.Second * 30}, nil
}

type XJoinDataSourceInstance struct {
	instance   *xjoin.XJoinDataSource
	client     client.Client
	log        xjoinlogger.Log
	parameters parameters.DataSourceParameters
}

func (i *XJoinDataSourceInstance) createDataSourcePipeline(name string, version string) (err error) {
	dataSourcePipeline := &unstructured.Unstructured{}
	dataSourcePipeline.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name + "." + version,
			"namespace": i.instance.Namespace,
		},
		"spec": map[string]interface{}{
			"Version":          version,
			"AvroSchema":       i.parameters.AvroSchema.String(),
			"DatabaseHostname": i.parameters.DatabaseHostname.String(),
			"DatabasePort":     i.parameters.DatabasePort.String(),
			"DatabaseName":     i.parameters.DatabaseName.String(),
			"DatabaseUsername": i.parameters.DatabaseUsername.String(),
			"DatabasePassword": i.parameters.DatabasePassword.String(),
			"Pause":            i.parameters.Pause.Bool(),
		},
	}
	dataSourcePipeline.SetGroupVersionKind(dataSourcePipelineGVK)

	blockOwnerDeletion := true
	controller := true
	ownerReference := metav1.OwnerReference{
		APIVersion:         i.instance.APIVersion,
		Kind:               i.instance.Kind,
		Name:               i.instance.GetName(),
		UID:                i.instance.GetUID(),
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
	dataSourcePipeline.SetOwnerReferences([]metav1.OwnerReference{ownerReference})

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = i.client.Create(ctx, dataSourcePipeline)
	return
}

func (i *XJoinDataSourceInstance) deleteDataSourcePipeline(name string, version string) (err error) {
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	dataSourcePipeline := &unstructured.Unstructured{}
	dataSourcePipeline.SetGroupVersionKind(dataSourcePipelineGVK)
	dataSourcePipeline.SetName(name + "." + version)
	return i.client.Delete(ctx, dataSourcePipeline)
}
