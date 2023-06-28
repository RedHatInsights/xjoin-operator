package index

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type XJoinIndexIteration struct {
	common.Iteration
	Parameters parameters.IndexParameters
}

func (i *XJoinIndexIteration) CreateIndexPipeline(name string, version string) (err error) {
	indexPipeline := unstructured.Unstructured{}

	spec := map[string]interface{}{
		"name":       name,
		"version":    version,
		"avroSchema": i.Parameters.AvroSchema.String(),
		"pause":      i.Parameters.Pause.Bool(),
		"ephemeral":  i.GetInstance().Spec.Ephemeral,
	}

	if len(i.Parameters.CustomSubgraphImages.Value().([]v1alpha1.CustomSubgraphImage)) != 0 {
		spec["customSubgraphImages"] = i.Parameters.CustomSubgraphImages.Value()
	}

	indexPipeline.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name + "." + version,
			"namespace": i.Iteration.Instance.GetNamespace(),
			"labels": map[string]interface{}{
				common.COMPONENT_NAME_LABEL: name,
			},
		},
		"spec": spec,
	}
	indexPipeline.SetGroupVersionKind(common.IndexPipelineGVK)

	err = i.CreateChildResource(indexPipeline, common.IndexGVK)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (i *XJoinIndexIteration) DeleteIndexPipeline(name string, version string) (err error) {
	err = i.DeleteResource(name+"."+version, common.IndexPipelineGVK)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (i XJoinIndexIteration) GetInstance() *v1alpha1.XJoinIndex {
	return i.Instance.(*v1alpha1.XJoinIndex)
}

func (i XJoinIndexIteration) GetFinalizerName() string {
	return "finalizer.xjoin.index.cloud.redhat.com"
}

func (i *XJoinIndexIteration) Finalize() (err error) {
	i.Log.Info("Starting finalizer")

	err = i.DeleteAllResourceTypeWithComponentName(common.IndexPipelineGVK, i.GetInstance().GetName())
	if err != nil {
		return errors.Wrap(err, 0)
	}

	controllerutil.RemoveFinalizer(i.Iteration.Instance, i.GetFinalizerName())

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = i.Client.Update(ctx, i.Iteration.Instance)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	i.Log.Info("Successfully finalized")
	return nil
}

func (i *XJoinIndexIteration) ReconcilePipeline() (err error) {
	child := NewIndexPipelineChild(i)
	err = i.ReconcileChild(child)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (i *XJoinIndexIteration) ReconcileChildren() (err error) {
	err = i.ReconcilePipeline()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}
