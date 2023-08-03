package datasource

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/common/labels"
	"github.com/redhatinsights/xjoin-operator/controllers/components"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type XJoinDataSourceIteration struct {
	common.Iteration
	Parameters parameters.DataSourceParameters
	Events     events.Events
}

func (i *XJoinDataSourceIteration) CreateDataSourcePipeline(name string, version string) (err error) {
	i.Events.Normal("Creating DatasourcePipeline", "name", name, "version", version)
	dataSourcePipeline := unstructured.Unstructured{}
	dataSourcePipeline.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name + "." + version,
			"namespace": i.Iteration.Instance.GetNamespace(),
			"labels": map[string]interface{}{
				labels.ComponentName:   components.DatasourcePipeline,
				labels.DatasourceName:  name,
				labels.PipelineVersion: version,
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
			"ephemeral":        i.GetInstance().Spec.Ephemeral,
		},
	}
	dataSourcePipeline.SetGroupVersionKind(common.DataSourcePipelineGVK)
	err = i.CreateChildResource(&dataSourcePipeline, common.DataSourceGVK)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (i *XJoinDataSourceIteration) DeleteDataSourcePipeline(name string, version string) (err error) {
	i.Events.Normal("Deleting DatasourcePipeline", "name", name, "version", version)
	err = i.DeleteResource(name+"."+version, common.DataSourcePipelineGVK)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (i *XJoinDataSourceIteration) ReconcilePipelines() (err error) {
	child := NewDataSourcePipelineChild(i)
	labelsMatch := client.MatchingLabels{
		labels.ComponentName:  components.DatasourcePipeline,
		labels.DatasourceName: i.GetInstance().GetName(),
	}
	err = i.ReconcileChild(child, labelsMatch)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (i *XJoinDataSourceIteration) Finalize() (err error) {
	i.Log.Info("Starting finalizer")

	labelsMatch := client.MatchingLabels{
		labels.ComponentName:  components.DatasourcePipeline,
		labels.DatasourceName: i.GetInstance().GetName(),
	}

	err = i.DeleteAllGVKsWithLabels(common.DataSourcePipelineGVK, labelsMatch)
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

func (i *XJoinDataSourceIteration) GetInstance() *v1alpha1.XJoinDataSource {
	return i.Instance.(*v1alpha1.XJoinDataSource)
}

func (i *XJoinDataSourceIteration) GetFinalizerName() string {
	return "finalizer.xjoin.datasource.cloud.redhat.com"
}
