package index

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type IndexPipelineChild struct {
	iteration *XJoinIndexIteration
}

func NewIndexPipelineChild(iteration *XJoinIndexIteration) *IndexPipelineChild {
	return &IndexPipelineChild{
		iteration: iteration,
	}
}

func (d *IndexPipelineChild) GetParentInstance() common.XJoinObject {
	return d.iteration.GetInstance()
}

func (d *IndexPipelineChild) Create(version string) (err error) {
	err = d.iteration.CreateIndexPipeline(d.iteration.GetInstance().GetName(), version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *IndexPipelineChild) Delete(version string) (err error) {
	err = d.iteration.DeleteIndexPipeline(d.iteration.GetInstance().GetName(), version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *IndexPipelineChild) GetGVK() (gvk schema.GroupVersionKind) {
	return common.IndexPipelineGVK
}
