package index

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type IndexValidatorChild struct {
	iteration *XJoinIndexIteration
}

func NewIndexValidatorChild(iteration *XJoinIndexIteration) *IndexValidatorChild {
	return &IndexValidatorChild{
		iteration: iteration,
	}
}

func (d *IndexValidatorChild) GetParentInstance() common.XJoinObject {
	return d.iteration.GetInstance()
}

func (d *IndexValidatorChild) Create(version string) (err error) {
	err = d.iteration.CreateIndexValidator(d.iteration.GetInstance().GetName(), version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *IndexValidatorChild) Delete(version string) (err error) {
	err = d.iteration.DeleteIndexValidator(d.iteration.GetInstance().GetName(), version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *IndexValidatorChild) GetGVK() (gvk schema.GroupVersionKind) {
	return common.IndexValidatorGVK
}
