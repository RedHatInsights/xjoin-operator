package datasource

import (
	"github.com/google/go-cmp/cmp"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type XJoinDataSourceInstance struct {
	Instance         *xjoin.XJoinDataSource
	Client           client.Client
	Log              xjoinlogger.Log
	Parameters       parameters.DataSourceParameters
	OriginalInstance xjoin.XJoinDataSource
}

var dataSourcePipelineGVK = schema.GroupVersionKind{
	Group:   "xjoin.cloud.redhat.com",
	Kind:    "XJoinDataSourcePipeline",
	Version: "v1alpha1",
}

func (i *XJoinDataSourceInstance) UpdateStatusAndRequeue() (reconcile.Result, error) {
	// Only issue status update if Reconcile actually modified Status
	// This prevents write conflicts between the controllers
	if !cmp.Equal(i.Instance.Status, i.OriginalInstance.Status) {
		i.Log.Debug("Updating status")

		ctx, cancel := utils.DefaultContext()
		defer cancel()

		if err := i.Client.Status().Update(ctx, i.Instance); err != nil {
			if k8errors.IsConflict(err) {
				i.Log.Error(err, "Status conflict")
				return reconcile.Result{}, err
			}

			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{RequeueAfter: time.Second * 30}, nil
}

func (i *XJoinDataSourceInstance) CreateDataSourcePipeline(name string, version string) (err error) {
	dataSourcePipeline := &unstructured.Unstructured{}
	dataSourcePipeline.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name + "." + version,
			"namespace": i.Instance.Namespace,
		},
		"spec": map[string]interface{}{
			"version":          version,
			"avroSchema":       i.Parameters.AvroSchema.String(),
			"databaseHostname": i.Parameters.DatabaseHostname.String(),
			"databasePort":     i.Parameters.DatabasePort.String(),
			"databaseName":     i.Parameters.DatabaseName.String(),
			"databaseUsername": i.Parameters.DatabaseUsername.String(),
			"databasePassword": i.Parameters.DatabasePassword.String(),
			"pause":            i.Parameters.Pause.Bool(),
		},
	}
	dataSourcePipeline.SetGroupVersionKind(dataSourcePipelineGVK)

	blockOwnerDeletion := true
	controller := true
	ownerReference := metav1.OwnerReference{
		APIVersion:         i.Instance.APIVersion,
		Kind:               i.Instance.Kind,
		Name:               i.Instance.GetName(),
		UID:                i.Instance.GetUID(),
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
	dataSourcePipeline.SetOwnerReferences([]metav1.OwnerReference{ownerReference})

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = i.Client.Create(ctx, dataSourcePipeline)
	return
}

func (i *XJoinDataSourceInstance) DeleteDataSourcePipeline(name string, version string) (err error) {
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	dataSourcePipeline := &unstructured.Unstructured{}
	dataSourcePipeline.SetGroupVersionKind(dataSourcePipelineGVK)
	dataSourcePipeline.SetName(name + "." + version)
	return i.Client.Delete(ctx, dataSourcePipeline)
}
