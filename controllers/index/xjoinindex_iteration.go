package index

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type XJoinIndexIteration struct {
	common.Iteration
	Parameters parameters.IndexParameters
}

func (i *XJoinIndexIteration) CreateIndexPipeline(name string, version string) (err error) {
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
			"name":       name,
			"version":    version,
			"avroSchema": i.Parameters.AvroSchema.String(),
			"pause":      i.Parameters.Pause.Bool(),
		},
	}
	dataSourcePipeline.SetGroupVersionKind(common.IndexPipelineGVK)

	err = i.CreateChildResource(dataSourcePipeline)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (i *XJoinIndexIteration) CreateIndexValidator(name string, version string) (err error) {
	indexValidator := unstructured.Unstructured{}
	indexValidator.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name + "." + version,
			"namespace": i.Iteration.Instance.GetNamespace(),
			"labels": map[string]interface{}{
				common.COMPONENT_NAME_LABEL: name,
			},
		},
		"spec": map[string]interface{}{
			"name":       name,
			"version":    version,
			"avroSchema": i.Parameters.AvroSchema.String(),
			"pause":      i.Parameters.Pause.Bool(),
		},
	}
	indexValidator.SetGroupVersionKind(common.IndexValidatorGVK)
	err = i.CreateChildResource(indexValidator)
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

func (i *XJoinIndexIteration) DeleteIndexValidator(name string, version string) (err error) {
	err = i.DeleteResource(name+"."+version, common.IndexValidatorGVK)
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

func (i *XJoinIndexIteration) ReconcileValidator() (err error) {
	child := NewIndexValidatorChild(i)
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
	err = i.ReconcileValidator()
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}
