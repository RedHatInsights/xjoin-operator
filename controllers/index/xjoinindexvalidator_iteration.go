package index

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/avro"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
)

const xjoinindexvalidatorFinalizer = "finalizer.xjoin.indexvalidator.cloud.redhat.com"

type XJoinIndexValidatorIteration struct {
	common.Iteration
	Parameters parameters.IndexParameters
}

func (i *XJoinIndexValidatorIteration) Finalize() (err error) {
	i.Log.Info("Starting finalizer")
	controllerutil.RemoveFinalizer(i.Instance, xjoinindexvalidatorFinalizer)

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = i.Client.Update(ctx, i.Instance)
	if err != nil {
		return
	}

	i.Log.Info("Successfully finalized")
	return nil
}

func (i *XJoinIndexValidatorIteration) Validate() (err error) {
	//Get index avro schema, references
	registry := avro.NewSchemaRegistry(
		avro.SchemaRegistryConnectionParams{
			Protocol: "http",
			Hostname: "confluent-schema-registry.test.svc",
			Port:     "8081",
		})

	registry.Init()

	//name+version of IndexValidator is identical to IndexPipeline
	references, err := registry.GetSchemaReferences("XJoinIndexPipeline." + i.GetInstance().GetName())
	if err != nil {
		return errors.Wrap(err, 0)
	}

	schemas := make(map[string]string)                                //map of schema names to schema definition
	dataSourcePipelines := make(map[string]unstructured.Unstructured) //map of schema names to dataSourcePipeline
	for _, ref := range references {
		//Get reference schemas
		refSchema, err := registry.GetSchema(ref.Subject)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		schemas[ref.Name] = refSchema

		//Get datasourcepipeline k8s object to get db connection info
		dataSourcePipeline := &unstructured.Unstructured{}
		dataSourcePipeline.SetGroupVersionKind(common.DataSourcePipelineGVK)
		dataSourcePipelineName := strings.Split(ref.Subject, "XJoinDataSourcePipeline.")[1]
		err = i.Client.Get(
			i.Context,
			client.ObjectKey{Name: dataSourcePipelineName, Namespace: i.GetInstance().Namespace},
			dataSourcePipeline)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		dataSourcePipelines[ref.Name] = *dataSourcePipeline
	}

	//use avro schema to map database rows, es documents
	//Validate loop
	return
}

func (i XJoinIndexValidatorIteration) GetInstance() *v1alpha1.XJoinIndexValidator {
	return i.Instance.(*v1alpha1.XJoinIndexValidator)
}
