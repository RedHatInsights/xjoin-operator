package index

import (
	"encoding/json"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/avro"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/riferrei/srclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type XJoinIndexPipelineIteration struct {
	common.Iteration
	Parameters parameters.IndexParameters
}

//ParseAvroSchemaReferences parses the Index's Avro Schema JSON to build a list of srclient.References
func (i *XJoinIndexPipelineIteration) ParseAvroSchemaReferences() (references []srclient.Reference, err error) {
	schemaString := i.GetInstance().Spec.AvroSchema
	var schemaObj avro.IndexSchema
	err = json.Unmarshal([]byte(schemaString), &schemaObj)
	if err != nil {
		return references, errors.Wrap(err, 0)
	}

	for _, field := range schemaObj.Fields {
		dataSourceName := strings.Split(field.Type, ".")[1] //TODO

		//get data source obj from field.Ref
		dataSource := &unstructured.Unstructured{}
		dataSource.SetGroupVersionKind(common.DataSourceGVK)
		err = i.Client.Get(i.Context, client.ObjectKey{Name: dataSourceName, Namespace: i.GetInstance().Namespace}, dataSource)
		if err != nil {
			return references, errors.Wrap(err, 0)
		}

		status := dataSource.Object["status"]
		if status == nil {
			err = errors.New("status missing from datasource")
			return references, errors.Wrap(err, 0)
		}
		statusMap := status.(map[string]interface{})
		//version := statusMap["activeVersion"] //TODO temporary
		version := statusMap["refreshingVersion"]
		if version == nil {
			err = errors.New("activeVersion missing from datasource.status")
			return references, errors.Wrap(err, 0)
		}
		versionString := version.(string)

		if versionString == "" {
			i.Log.Info("Data source is not ready yet. It has no active version.",
				"datasource", dataSourceName)
			return
		}

		ref := srclient.Reference{
			Name:    field.Type,
			Subject: "xjoindatasourcepipeline." + dataSourceName + "." + versionString + "-value",
			Version: 1,
		}

		references = append(references, ref)
	}

	return
}

func (i *XJoinIndexPipelineIteration) AvroSubjectToKafkaTopic(avroSubject string) (kafkaTopic string) {
	//avro subjects have a -value suffix while kafka topics do not
	//e.g. xjoindatasourcepipeline.hosts.123456789-value
	return strings.Split(avroSubject, "-")[0]
}

func (i *XJoinIndexPipelineIteration) ParseSourceTopics(references []srclient.Reference) (sourceTopics string) {
	for idx, reference := range references {
		if idx != 0 {
			sourceTopics = sourceTopics + ","
		}

		sourceTopics = sourceTopics + strings.ToLower(i.AvroSubjectToKafkaTopic(reference.Subject))
	}
	return
}

func (i XJoinIndexPipelineIteration) GetInstance() *v1alpha1.XJoinIndexPipeline {
	return i.Instance.(*v1alpha1.XJoinIndexPipeline)
}
