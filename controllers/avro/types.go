package avro

import (
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"reflect"
	"strings"
)

type Schema struct {
	XJoinType       string           `json:"xjoin.type,omitempty"`
	Type            TypeWrapper      `json:"type,omitempty"`
	Name            string           `json:"name,omitempty"`
	Namespace       string           `json:"namespace,omitempty"`
	Fields          []Field          `json:"fields,omitempty"`
	ConnectName     string           `json:"connect.name,omitempty"`
	Transformations []Transformation `json:"xjoin.transformations,omitempty"`
}

type Type struct {
	Type             string      `json:"type,omitempty"`
	Name             string      `json:"name,omitempty"`  //required for type=record
	Items            TypeWrapper `json:"items,omitempty"` //required for type=array
	Fields           []Field     `json:"fields,omitempty"`
	XJoinType        string      `json:"xjoin.type,omitempty"`
	XJoinFields      []Field     `json:"xjoin.fields,omitempty"`
	ConnectVersion   int         `json:"connect.version,omitempty"`
	ConnectName      string      `json:"connect.name,omitempty"`
	XJoinCase        string      `json:"xjoin.case,omitempty"`
	XJoinEnumeration bool        `json:"xjoin.enumeration,omitempty"`
	XJoinPrimaryKey  bool        `json:"xjoin.primary.key,omitempty"`
}

type Field struct {
	Name       string      `json:"name,omitempty"`
	Type       TypeWrapper `json:"type,omitempty"`
	Default    string      `json:"default,omitempty"`
	XJoinIndex *bool       `json:"xjoin.index,omitempty"`
}

type Transformation struct {
	Type        string                 `json:"transformation,omitempty"`
	InputField  string                 `json:"input.field,omitempty"`
	OutputField string                 `json:"output.field,omitempty"`
	Parameters  map[string]interface{} `json:"transformation.parameters,omitempty"`
}

type TypeWrapper []Type

func (t TypeWrapper) MarshalJSON() ([]byte, error) {
	if len(t) == 1 {
		typeObj := t[0]
		//if only type is set, marshal type as a plain string
		if typeObj.XJoinFields == nil &&
			typeObj.XJoinType == "" &&
			typeObj.Fields == nil &&
			typeObj.ConnectName == "" &&
			typeObj.ConnectVersion == 0 &&
			typeObj.XJoinCase == "" {

			typeString := typeObj.Type
			return json.Marshal(typeString)
		} else {
			//only one type so marshal as a non array type object
			return json.Marshal(typeObj)
		}
	} else if len(t) > 1 {
		//multiple types so marshal as an array of types
		//copy into a new slice to avoid infinite loop
		var typeArray []Type
		for _, typeElem := range t {
			typeArray = append(typeArray, typeElem)
		}
		return json.Marshal(typeArray)
	} else {
		return []byte(""), nil
	}

}

func (t *TypeWrapper) UnmarshalJSON(typeBytes []byte) (err error) {
	//first attempt to unmarshal into a string
	//then attempt to unmarshal into a Type
	//then attempt to unmarshal into []Type
	//if that fails return nil
	var typeString string
	err = json.Unmarshal(typeBytes, &typeString)

	if err == nil {
		var typeObj Type
		typeObj = Type{
			Type: typeString,
		}
		*t = []Type{typeObj}
	} else {
		var typeObj Type
		err = json.Unmarshal(typeBytes, &typeObj)
		if err == nil {
			*t = []Type{typeObj}
		} else {
			//could be ["null", {"type": "string}]
			//or [{"type": null}, {"type": string"}]
			var interfaceArr []interface{}
			err = json.Unmarshal(typeBytes, &interfaceArr)

			if err == nil {
				var typeArr []Type
				for _, typeInterface := range interfaceArr {
					var typeObject Type
					if reflect.TypeOf(typeInterface).Kind() == reflect.String {
						typeObject = Type{
							Type: typeInterface.(string),
						}
					} else if reflect.TypeOf(typeInterface).Kind() == reflect.Map {
						typeJson, err := json.Marshal(typeInterface)
						if err != nil {
							return errors.Wrap(err, 0)
						}
						err = json.Unmarshal(typeJson, &typeObject)
						if err != nil {
							return errors.Wrap(err, 0)
						}
					}
					typeArr = append(typeArr, typeObject)
				}

				*t = typeArr
			}
		}
	}

	return nil
}

func (s *Schema) AddField(name string, newField Field) (err error) {
	nodeNames := strings.Split(name, ".")

	currentNodeChildren := s.Fields
	for idx, nodeName := range nodeNames {
		node, err := s.getFieldByName(nodeName, currentNodeChildren)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		if idx < len(nodeNames)-2 {
			currentNodeChildren = node.Type[0].Fields //TODO handle multiple types, null check
		} else {
			//found the node to add field to
			node.Type[0].Fields = append(node.Type[0].Fields, newField)
			break
		}
	}

	return
}

func (s *Schema) GetFieldByName(name string) (node Field, err error) {
	nodeNames := strings.Split(name, ".")

	currentNodeChildren := s.Fields
	for idx, nodeName := range nodeNames {
		node, err = s.getFieldByName(nodeName, currentNodeChildren)
		if err != nil {
			return node, errors.Wrap(err, 0)
		}

		if idx < len(nodeNames)-1 {
			currentNodeChildren = node.Type[0].Fields //TODO handle multiple types, null check
		}
	}

	return
}

func (s Schema) getFieldByName(name string, fields []Field) (fieldNode Field, err error) {
	for _, field := range fields {
		if field.Name == name {
			return field, nil
		}
	}

	return fieldNode, errors.Wrap(errors.New(fmt.Sprintf("Field %s not found in schema", name)), 0)
}
