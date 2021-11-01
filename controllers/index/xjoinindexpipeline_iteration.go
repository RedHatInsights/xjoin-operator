package index

import (
	"container/list"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/avro"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/riferrei/srclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"reflect"
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

//ParseAvroSchema transforms an avro schema into elasticsearch mapping properties and a list of jsonFields
func (i XJoinIndexPipelineIteration) ParseAvroSchema(
	avroSchema map[string]interface{}) (properties string, jsonFields []string, err error) {

	if avroSchema["fields"] == nil {
		return properties, jsonFields, errors.Wrap(errors.New("fields missing from avro schema"), 0)
	}

	fields := avroSchema["fields"].([]interface{})
	esProperties, jsonFields, err := parseAvroFields(fields, list.List{})
	if err != nil {
		return properties, jsonFields, errors.Wrap(err, 0)
	}

	propertiesBytes, err := json.Marshal(esProperties)
	if err != nil {
		return properties, jsonFields, errors.Wrap(err, 0)
	}

	return string(propertiesBytes), jsonFields, nil
}

func parseAvroFields(avroFields []interface{}, parent list.List) (map[string]interface{}, []string, error) {
	esProperties := make(map[string]interface{})
	var jsonFields []string

	for _, f := range avroFields {
		avroField := f.(map[string]interface{})
		esProperty := make(map[string]interface{})

		if avroField["xjoin.index"] != nil && avroField["xjoin.index"].(bool) == false {
			continue
		}

		//determine this field's type
		var avroFieldTypeObject map[string]interface{}
		if reflect.TypeOf(avroField["type"]).Kind() == reflect.Slice {
			/* This IF statement handles an array type, e.g.
			"type": [
				{
					"type": "null"
				},
				{
					"type": "string",
					"xjoin.type": "string"
				}
			]
			*/
			avroFieldTypeArray := avroField["type"].([]interface{})
			if reflect.TypeOf(avroFieldTypeArray[0]).Kind() != reflect.String || avroFieldTypeArray[0].(string) != "null" {
				return nil, nil, errors.Wrap(errors.New("avro field's type must be [null, type_object{}]"), 0)
			}

			if reflect.TypeOf(avroFieldTypeArray[1]).Kind() == reflect.Map {
				avroFieldTypeObject = avroFieldTypeArray[1].(map[string]interface{})
			} else {
				return nil, nil, errors.Wrap(errors.New(
					"avro field's type must be an object or union of [null, object{}]"), 0)
			}

		} else if reflect.TypeOf(avroField["type"]).Kind() == reflect.Map {
			/* This IF statement handles an object type, e.g.
			   "type": {
			       "type": "string",
			       "xjoin.type": "date_nanos",
				}
			*/
			avroFieldTypeObject = avroField["type"].(map[string]interface{})
		} else {
			return nil, nil, errors.Wrap(
				errors.New(
					fmt.Sprintf(
						"avro field's type must be an object or union of [null, object{}], kind: %s", reflect.TypeOf(avroField["type"]).Kind())), 0)
		}

		esProperty["type"] = avroTypeToElasticsearchType(avroFieldTypeObject["xjoin.type"].(string))
		esProperty, err := parseXJoinFlags(avroFieldTypeObject, esProperty)
		if err != nil {
			return nil, nil, errors.Wrap(err, 0)
		}

		//find json fields which need to be transformed from a string
		if avroFieldTypeObject["xjoin.type"] == "json" && avroFieldTypeObject["type"] == "string" {
			jsonFieldName := ""
			for parentField := parent.Front(); parentField != nil; parentField = parentField.Next() {
				jsonFieldName = parentField.Value.(string) + "."

			}
			jsonFieldName = jsonFieldName + avroField["name"].(string)
			jsonFields = append(jsonFields, jsonFieldName)
		}

		//recurse through nested object types
		if esProperty["type"] == "object" {
			//nested json objects are "type: string", "xjoin.type: json" with xjoin.fields
			//top level records are "type: record" with standard avro fields
			var nestedFields []interface{}
			if avroFieldTypeObject["fields"] != nil {
				nestedFields = avroFieldTypeObject["fields"].([]interface{})
			} else if avroFieldTypeObject["xjoin.fields"] != nil {
				nestedFields = avroFieldTypeObject["xjoin.fields"].([]interface{})
			}

			if nestedFields != nil {
				newParent := parent
				newParent.PushFront(avroField["name"])
				nestedProperties, nestedJsonFields, err :=
					parseAvroFields(nestedFields, newParent)
				if err != nil {
					return nil, nil, errors.Wrap(err, 0)
				}
				esProperty["properties"] = nestedProperties
				jsonFields = append(jsonFields, nestedJsonFields...)
			}
		}

		esProperties[avroField["name"].(string)] = esProperty
	}
	return esProperties, jsonFields, nil
}

func avroTypeToElasticsearchType(avroType string) (esType string) {
	switch strings.ToLower(avroType) {
	case "date_nanos":
		esType = "date_nanos"
	case "string":
		esType = "keyword"
	case "boolean":
		esType = "boolean"
	case "json":
		esType = "object"
	case "record":
		esType = "object"
	case "reference":
		esType = "object"
	default:
		esType = "keyword" //TODO should this be default or error?
	}

	return
}

func parseXJoinFlags(avroField map[string]interface{}, esProperty map[string]interface{}) (map[string]interface{}, error) {
	if avroField["xjoin.case"] != nil {
		if avroField["xjoin.type"] != "string" {
			return nil, errors.Wrap(errors.New("xjoin.case can only be applied to string fields"), 0)
		}

		if avroField["xjoin.case"] == "insensitive" {
			fields := make(map[string]interface{})
			lowercaseField := make(map[string]interface{})
			lowercaseField["type"] = "keyword"
			lowercaseField["normalizer"] = "case_insensitive"
			fields["lowercase"] = lowercaseField
			esProperty["fields"] = fields
		} else if avroField["xjoin.case"] != "sensitive" {
			return nil, errors.Wrap(errors.New("xjoin.case must be one of [insensitive, sensitive]"), 0)
		}
	}

	return esProperty, nil
}

func (i XJoinIndexPipelineIteration) GetInstance() *v1alpha1.XJoinIndexPipeline {
	return i.Instance.(*v1alpha1.XJoinIndexPipeline)
}
