package datasource

import (
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type XJoinDataSourceIteration struct {
	common.Iteration
	Parameters parameters.DataSourceParameters
}

var dataSourcePipelineGVK = schema.GroupVersionKind{
	Group:   "xjoin.cloud.redhat.com",
	Kind:    "XJoinDataSourcePipeline",
	Version: "v1alpha1",
}

func (i *XJoinDataSourceIteration) CreateDataSourcePipeline(name string, version string) (err error) {
	dataSourcePipeline := unstructured.Unstructured{}
	dataSourcePipeline.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name + "." + version,
			"namespace": i.Iteration.Instance.GetNamespace(),
		},
		"spec": map[string]interface{}{
			"version":          version,
			"avroSchema":       i.Parameters.AvroSchema.String(),
			"databaseHostname": i.Parameters.DatabaseHostname.String(),
			"databasePort":     i.Parameters.DatabasePort.String(),
			"databaseName":     i.Parameters.DatabaseName.String(),
			"databaseUsername": i.Parameters.DatabaseUsername.String(),
			"databasePassword": i.Parameters.DatabasePassword.String(),
			"pause":            i.Parameters.Pause.Bool(),
		},
	}
	dataSourcePipeline.SetGroupVersionKind(dataSourcePipelineGVK)
	return i.CreateChildResource(dataSourcePipeline)
}

func (i *XJoinDataSourceIteration) DeleteDataSourcePipeline(name string, version string) (err error) {
	return i.DeleteResource(name+"."+version, dataSourcePipelineGVK)
}

func (i XJoinDataSourceIteration) GetInstance() *v1alpha1.XJoinDataSource {
	return i.Instance.(*v1alpha1.XJoinDataSource)
}
