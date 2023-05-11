package avro

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-errors/errors"
	. "github.com/redhatinsights/xjoin-go-lib/pkg/avro"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	"github.com/riferrei/srclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OBJECT_TO_ARRAY_OF_OBJECTS = "object_to_array_of_objects"
	OBJECT_TO_ARRAY_OF_STRINGS = "object_to_array_of_strings"
)

// IndexAvroSchema is a completely parsed representation of a xjoinindex avro schema
type IndexAvroSchema struct {
	AvroSchema       Schema
	AvroSchemaString string
	References       []srclient.Reference
	ESProperties     string
	JSONFields       []string
	SourceTopics     string
}

type IndexAvroSchemaParser struct {
	AvroSchema      string
	Client          client.Client
	Context         context.Context
	Namespace       string
	SchemaNamespace string
	Log             log.Log
	SchemaRegistry  *schemaregistry.ConfluentClient
}

// Parse AvroSchema string into various structures represented by IndexAvroSchema to be used in component creation
func (d *IndexAvroSchemaParser) Parse() (indexAvroSchema IndexAvroSchema, err error) {
	indexAvroSchema.References, err = d.parseAvroSchemaReferences()
	if err != nil {
		return indexAvroSchema, errors.Wrap(err, 0)
	}

	indexAvroSchema.SourceTopics = d.ParseSourceTopics(indexAvroSchema.References)

	indexAvroSchema.AvroSchema, err = d.expandReferences(d.AvroSchema, indexAvroSchema.References)
	if err != nil {
		return indexAvroSchema, errors.Wrap(err, 0)
	}

	indexAvroSchema.AvroSchema, err = d.applyTransformations(indexAvroSchema.AvroSchema)
	if err != nil {
		return indexAvroSchema, errors.Wrap(err, 0)
	}

	indexAvroSchema.ESProperties, indexAvroSchema.JSONFields, err = d.transformToES(indexAvroSchema.AvroSchema)
	if err != nil {
		return indexAvroSchema, errors.Wrap(err, 0)
	}

	indexAvroSchema.AvroSchema.Name = "Value"
	indexAvroSchema.AvroSchema.Namespace = d.SchemaNamespace

	avroSchemaString, err := json.Marshal(indexAvroSchema.AvroSchema)
	if err != nil {
		return indexAvroSchema, errors.Wrap(err, 0)
	}
	indexAvroSchema.AvroSchemaString = string(avroSchemaString)

	return
}

// applyTransformations adds the fields defined in xjoin.transformations to the Schema
func (d *IndexAvroSchemaParser) applyTransformations(avroSchema Schema) (transformedAvroSchema Schema, err error) {
	for _, transformation := range avroSchema.Transformations {
		switch transformation.Type {
		case OBJECT_TO_ARRAY_OF_OBJECTS:
			avroSchema, err = d.applyObjectToArrayOfObjectsTransformation(avroSchema, transformation)
			if err != nil {
				return transformedAvroSchema, errors.Wrap(err, 0)
			}
		case OBJECT_TO_ARRAY_OF_STRINGS:
			avroSchema, err = d.applyObjectToArrayOfStringsTransformation(avroSchema, transformation)
			if err != nil {
				return transformedAvroSchema, errors.Wrap(err, 0)
			}
		}
	}

	return avroSchema, nil
}

func (d *IndexAvroSchemaParser) applyObjectToArrayOfStringsTransformation(
	avroSchema Schema, transformation Transformation) (Schema, error) {

	stringType := Type{
		Type: "string",
	}

	fieldNodes := strings.Split(transformation.OutputField, ".")
	fieldName := fieldNodes[len(fieldNodes)-1]
	fieldType := Type{
		Type:      "array",
		XJoinType: "array",
		Items:     []Type{stringType},
	}
	field := Field{
		Name: fieldName,
		Type: []Type{fieldType},
	}
	err := avroSchema.AddField(transformation.OutputField, field)
	if err != nil {
		return avroSchema, errors.Wrap(err, 0)
	}

	return avroSchema, nil
}

func (d *IndexAvroSchemaParser) applyObjectToArrayOfObjectsTransformation(
	avroSchema Schema, transformation Transformation) (Schema, error) {

	if transformation.Parameters["keys"] == nil || reflect.TypeOf(transformation.Parameters["keys"]).Kind() != reflect.Slice {
		return avroSchema, errors.Wrap(errors.New(fmt.Sprintf(
			"keys field missing from transformation: %s, output_field: %s", OBJECT_TO_ARRAY_OF_OBJECTS, transformation.OutputField)), 0)
	}

	var childFields []Field

	for _, key := range transformation.Parameters["keys"].([]interface{}) {
		if reflect.TypeOf(key).Kind() != reflect.String {
			return avroSchema, errors.Wrap(errors.New(fmt.Sprintf(
				"keys field must be an array of strings, output_field: %s", transformation.OutputField)), 0)
		}

		nullType := Type{
			Type: "null",
		}

		childType := Type{
			Type:      "string",
			XJoinType: "string",
		}
		childFields = append(childFields, Field{
			Name: key.(string),
			Type: []Type{nullType, childType},
		})
	}

	fieldNodes := strings.Split(transformation.OutputField, ".")
	fieldName := fieldNodes[len(fieldNodes)-1]

	childType := Type{
		Type:      "record",
		XJoinType: "json",
		Name:      "children",
		Fields:    childFields,
	}

	fieldType := Type{
		Type:      "array",
		XJoinType: "json",
		Name:      fieldName,
		Items:     []Type{childType},
	}

	field := Field{
		Name: fieldName,
		Type: []Type{fieldType},
	}

	err := avroSchema.AddField(transformation.OutputField, field)
	if err != nil {
		return avroSchema, errors.Wrap(err, 0)
	}

	return avroSchema, nil
}

// ParseAvroSchemaReferences parses the Index's Avro Schema JSON to build a list of srclient.References
func (d *IndexAvroSchemaParser) parseAvroSchemaReferences() (references []srclient.Reference, err error) {
	schemaString := d.AvroSchema
	var schemaObj Schema
	err = json.Unmarshal([]byte(schemaString), &schemaObj)
	if err != nil {
		d.Log.Error(err, "Unable to parse avro schema as JSON", "Schema", schemaString)
		return references, errors.Wrap(err, 0)
	}

	for _, field := range schemaObj.Fields {
		if len(field.Type) == 0 || len(strings.Split(field.Type[0].Type, ".")) < 2 {
			return references, errors.Wrap(errors.New("unable to parse dataSourceName from avro schema fields"), 0)
		}
		dataSourceName := strings.Split(field.Type[0].Type, ".")[1]

		//get data source obj from field.Ref
		dataSource := &unstructured.Unstructured{}
		dataSource.SetGroupVersionKind(common.DataSourceGVK)
		err = d.Client.Get(d.Context, client.ObjectKey{Name: dataSourceName, Namespace: d.Namespace}, dataSource)
		if err != nil {
			return references, errors.Wrap(err, 0)
		}

		status := dataSource.Object["status"]
		if status == nil {
			err = errors.New("status missing from datasource")
			return references, errors.Wrap(err, 0)
		}
		statusMap := status.(map[string]interface{})

		//determine which datasource version to use
		activeVersion := statusMap["activeVersion"].(string)
		refreshingVersion := statusMap["refreshingVersion"].(string)
		activeVersionIsValid := statusMap["activeVersionIsValid"].(bool)
		var chosenVersion string

		if activeVersion != "" && activeVersionIsValid {
			chosenVersion = statusMap["activeVersion"].(string)
		} else if refreshingVersion != "" {
			chosenVersion = refreshingVersion
		} else {
			return nil, errors.Wrap(errors.New(
				"Datasource ("+dataSourceName+") is not ready yet. It has no active or refreshing version."), 0)
		}

		ref := srclient.Reference{
			Name:    field.Type[0].Type,
			Subject: "xjoindatasourcepipeline." + dataSourceName + "." + chosenVersion + "-value",
			Version: 1,
		}

		references = append(references, ref)
	}

	return
}

// ParseAvroSchema transforms an avro schema into elasticsearch mapping properties and a list of jsonFields
func (d *IndexAvroSchemaParser) transformToES(avroSchema Schema) (properties string, jsonFields []string, err error) {

	if avroSchema.Fields == nil {
		return properties, jsonFields, errors.Wrap(errors.New("fields property is missing from avro schema"), 0)
	}

	esProperties, jsonFields, err := parseAvroFields(avroSchema.Fields, list.List{})
	if err != nil {
		return properties, jsonFields, errors.Wrap(err, 0)
	}

	propertiesBytes, err := json.Marshal(esProperties)
	if err != nil {
		return properties, jsonFields, errors.Wrap(err, 0)
	}

	return string(propertiesBytes), jsonFields, nil
}

func parseAvroFields(avroFields []Field, parent list.List) (map[string]interface{}, []string, error) {
	esProperties := make(map[string]interface{})
	var jsonFields []string

	for _, avroField := range avroFields {
		esProperty := make(map[string]interface{})

		if avroField.XJoinIndex != nil && !*avroField.XJoinIndex {
			continue
		}

		//determine this field's type
		var avroFieldType Type
		if len(avroField.Type) > 1 {
			avroFieldType = avroField.Type[1]
		} else {
			avroFieldType = avroField.Type[0]
		}

		esProperty["type"] = avroTypeToElasticsearchType(avroFieldType)
		esProperty, err := parseXJoinFlags(avroFieldType, esProperty)
		if err != nil {
			return nil, nil, errors.Wrap(err, 0)
		}

		//find json fields which need to be transformed from a string
		if avroFieldType.XJoinType == "json" && avroFieldType.Type == "string" {
			jsonFieldName := ""
			for parentField := parent.Front(); parentField != nil; parentField = parentField.Next() {
				jsonFieldName = parentField.Value.(string) + "."

			}
			jsonFieldName = jsonFieldName + avroField.Name
			jsonFields = append(jsonFields, jsonFieldName)
		}

		//recurse through nested object types
		if esProperty["type"] == "object" {
			//nested json objects are "type: string", "xjoin.type: json" with xjoin.fields
			//top level records are "type: record" with standard avro fields
			var nestedFields []Field
			if avroFieldType.Fields != nil {
				nestedFields = avroFieldType.Fields
			} else if avroFieldType.XJoinFields != nil {
				nestedFields = avroFieldType.XJoinFields
			}

			if nestedFields != nil {
				newParent := parent
				newParent.PushFront(avroField.Name)
				nestedProperties, nestedJsonFields, err :=
					parseAvroFields(nestedFields, newParent)
				if err != nil {
					return nil, nil, errors.Wrap(err, 0)
				}
				esProperty["properties"] = nestedProperties
				jsonFields = append(jsonFields, nestedJsonFields...)
			}
		}

		esProperties[avroField.Name] = esProperty
	}
	return esProperties, jsonFields, nil
}

func avroTypeToElasticsearchType(avroType Type) (esType string) {
	typeString := avroType.XJoinType
	if avroType.XJoinType == "array" {
		typeString = avroType.Items[0].Type //TODO: handle multiple items?
	}

	switch strings.ToLower(typeString) {
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

func parseXJoinFlags(avroFieldType Type, esProperty map[string]interface{}) (map[string]interface{}, error) {
	if avroFieldType.XJoinCase != "" {
		if avroFieldType.XJoinType != "string" {
			return nil, errors.Wrap(errors.New("xjoin.case can only be applied to string fields"), 0)
		}

		if avroFieldType.XJoinCase == "insensitive" {
			fields := make(map[string]interface{})
			lowercaseField := make(map[string]interface{})
			lowercaseField["type"] = "keyword"
			lowercaseField["normalizer"] = "case_insensitive"
			fields["lowercase"] = lowercaseField
			esProperty["fields"] = fields
		} else if avroFieldType.XJoinCase != "sensitive" {
			return nil, errors.Wrap(errors.New("xjoin.case must be one of [insensitive, sensitive]"), 0)
		}
	}

	return esProperty, nil
}

// ExpandReferences retrieves the full schema for each xjoinref field
func (d *IndexAvroSchemaParser) expandReferences(baseSchema string, references []srclient.Reference) (fullSchema Schema, err error) {
	err = json.Unmarshal([]byte(baseSchema), &fullSchema)
	if err != nil {
		return fullSchema, errors.Wrap(err, 0)
	}

	//TODO handle type array instead of assuming type[0]
	for idx, field := range fullSchema.Fields {
		if field.Type[0].XJoinType == "reference" {
			ref, err := findReferenceByType(references, field.Type[0].Type)
			if err != nil {
				return fullSchema, errors.Wrap(err, 0)
			}
			refSchemaString, err := d.SchemaRegistry.GetSchema(ref.Subject)
			if err != nil {
				return fullSchema, errors.Wrap(err, 0)
			}

			var refSchemaType Type
			err = json.Unmarshal([]byte(refSchemaString), &refSchemaType)
			if err != nil {
				return fullSchema, errors.Wrap(err, 0)
			}

			refSchemaType.XJoinType = field.Type[0].XJoinType
			refSchemaType.Name = ref.Name
			fullSchema.Fields[idx].Type = []Type{refSchemaType}
		}
	}

	return
}

func findReferenceByType(references []srclient.Reference, refType string) (srclient.Reference, error) {
	for _, ref := range references {
		if ref.Name == refType {
			return ref, nil
		}
	}
	return srclient.Reference{}, errors.Wrap(errors.New("reference "+refType+"not found in list of references"), 0)
}

func (d *IndexAvroSchemaParser) AvroSubjectToKafkaTopic(avroSubject string) (kafkaTopic string) {
	//avro subjects have a -value suffix while kafka topics do not
	//e.g. xjoindatasourcepipeline.hosts.123456789-value
	return strings.Split(avroSubject, "-")[0]
}

func (d *IndexAvroSchemaParser) ParseSourceTopics(references []srclient.Reference) (sourceTopics string) {
	for idx, reference := range references {
		if idx != 0 {
			sourceTopics = sourceTopics + ","
		}

		sourceTopics = sourceTopics + strings.ToLower(d.AvroSubjectToKafkaTopic(reference.Subject))
	}
	return
}
