package datasource

import (
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
)

type XJoinDataSourcePipelineIteration struct {
	common.Iteration
	Parameters parameters.DataSourceParameters
}

func (i XJoinDataSourcePipelineIteration) GetInstance() *v1alpha1.XJoinDataSourcePipeline {
	return i.Instance.(*v1alpha1.XJoinDataSourcePipeline)
}
