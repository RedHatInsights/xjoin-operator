package index

import (
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
)

type XJoinIndexPipelineIteration struct {
	common.Iteration
	Parameters parameters.IndexParameters
}

func (i XJoinIndexPipelineIteration) GetInstance() *v1alpha1.XJoinIndexPipeline {
	return i.Instance.(*v1alpha1.XJoinIndexPipeline)
}
