package datasource

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type DataSourcePipelineChild struct {
	iteration *XJoinDataSourceIteration
}

func NewDataSourcePipelineChild(iteration *XJoinDataSourceIteration) *DataSourcePipelineChild {
	return &DataSourcePipelineChild{
		iteration: iteration,
	}
}

func (d *DataSourcePipelineChild) GetParentInstance() common.XJoinObject {
	return d.iteration.GetInstance()
}

func (d *DataSourcePipelineChild) Create(version string) (err error) {
	err = d.iteration.CreateDataSourcePipeline(d.iteration.GetInstance().GetName(), version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *DataSourcePipelineChild) Delete(version string) (err error) {
	err = d.iteration.DeleteDataSourcePipeline(d.iteration.GetInstance().GetName(), version)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (d *DataSourcePipelineChild) GetGVK() (gvk schema.GroupVersionKind) {
	return common.DataSourcePipelineGVK
}
