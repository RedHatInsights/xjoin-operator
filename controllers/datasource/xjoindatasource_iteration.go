package datasource

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type XJoinDataSourceIteration struct {
	common.Iteration
	Parameters parameters.DataSourceParameters
}

func (i *XJoinDataSourceIteration) CreateDataSourcePipeline(name string, version string) (err error) {
	dataSourcePipeline := unstructured.Unstructured{}
	dataSourcePipeline.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name + "." + version,
			"namespace": i.Iteration.Instance.GetNamespace(),
			"labels": map[string]interface{}{
				common.COMPONENT_NAME_LABEL: name,
			},
		},
		"spec": map[string]interface{}{
			"name":             name,
			"version":          version,
			"avroSchema":       i.Parameters.AvroSchema.String(),
			"databaseHostname": i.GetInstance().Spec.DatabaseHostname,
			"databasePort":     i.GetInstance().Spec.DatabasePort,
			"databaseName":     i.GetInstance().Spec.DatabaseName,
			"databaseUsername": i.GetInstance().Spec.DatabaseUsername,
			"databasePassword": i.GetInstance().Spec.DatabasePassword,
			"databaseTable":    i.GetInstance().Spec.DatabaseTable,
			"pause":            i.Parameters.Pause.Bool(),
		},
	}
	dataSourcePipeline.SetGroupVersionKind(common.DataSourcePipelineGVK)
	err = i.CreateChildResource(dataSourcePipeline)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (i *XJoinDataSourceIteration) DeleteDataSourcePipeline(name string, version string) (err error) {
	err = i.DeleteResource(name+"."+version, common.DataSourcePipelineGVK)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (i *XJoinDataSourceIteration) ReconcilePipelines() (err error) {
	child := NewDataSourcePipelineChild(i)
	err = i.ReconcileChild(child)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (i *XJoinDataSourceIteration) Finalize() (err error) {
	i.Log.Info("Starting finalizer")

	err = i.DeleteAllResourceTypeWithComponentName(common.DataSourcePipelineGVK, i.GetInstance().GetName())
	if err != nil {
		return errors.Wrap(err, 0)
	}

	controllerutil.RemoveFinalizer(i.Instance, i.GetFinalizerName())
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = i.Client.Update(ctx, i.Instance)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	i.Log.Info("Successfully finalized")
	return nil
}

func (i XJoinDataSourceIteration) GetInstance() *v1alpha1.XJoinDataSource {
	return i.Instance.(*v1alpha1.XJoinDataSource)
}

func (i XJoinDataSourceIteration) GetFinalizerName() string {
	return "finalizer.xjoin.datasource.cloud.redhat.com"
}
