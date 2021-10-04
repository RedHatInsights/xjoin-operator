package datasource

import (
	"encoding/json"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/avro"
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

func (i XJoinDataSourcePipelineIteration) ParseAvroSchema() (schema string, err error) {
	schemaString := i.GetInstance().Spec.AvroSchema
	var schemaObj avro.SourceSchema
	err = json.Unmarshal([]byte(schemaString), &schemaObj)
	if err != nil {
		return schema, errors.Wrap(err, 0)
	}

	schemaObj.Name = "xjoin.datasource." + i.GetInstance().Spec.Name

	schemaBytes, err := json.Marshal(schemaObj)
	if err != nil {
		return schema, errors.Wrap(err, 0)
	}

	return string(schemaBytes), err
}
