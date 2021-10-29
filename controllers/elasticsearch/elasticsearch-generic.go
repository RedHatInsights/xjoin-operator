package elasticsearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/go-errors/errors"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"text/template"
)

type GenericElasticsearch struct {
	Parameters map[string]interface{}
	Client     *elasticsearch.Client
	Context    context.Context
}

type GenericElasticSearchParameters struct {
	Url        string
	Username   string
	Password   string
	Parameters map[string]interface{}
	Context    context.Context
}

func NewGenericElasticsearch(params GenericElasticSearchParameters) (*GenericElasticsearch, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{params.Url},
		Username:  params.Username,
		Password:  params.Password,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	es := GenericElasticsearch{
		Parameters: params.Parameters,
		Client:     client,
		Context:    params.Context,
	}

	return &es, nil
}

func (es GenericElasticsearch) IndexExists(indexName string) (bool, error) {
	res, err := es.Client.Indices.Exists([]string{indexName})
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	responseCode, _, err := parseResponse(res)
	if err != nil && responseCode != 404 {
		return false, errors.Wrap(err, 0)
	} else if responseCode == 404 {
		return false, nil
	}

	return true, nil
}

func (es *GenericElasticsearch) DeleteIndexByFullName(index string) error {
	if index == "" {
		return nil
	}

	res, err := es.Client.Indices.Delete([]string{index})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	responseCode, _, err := parseResponse(res)
	if err != nil && responseCode != 404 {
		return errors.Wrap(err, 0)
	}

	return nil
}

func (es GenericElasticsearch) ListIndicesForPrefix(prefix string) ([]string, error) {
	req := esapi.CatIndicesRequest{
		Format: "JSON",
		Index:  []string{prefix + ".*"},
		H:      []string{"index"},
	}
	res, err := req.Do(es.Context, es.Client)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	defer res.Body.Close()

	byteValue, _ := ioutil.ReadAll(res.Body)

	var indicesJSON []map[string]string
	err = json.Unmarshal(byteValue, &indicesJSON)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	var indices []string
	for _, index := range indicesJSON {
		indices = append(indices, index["index"])
	}

	return indices, nil
}

func (es GenericElasticsearch) CreateIndex(
	indexName string, indexTemplate string, avroSchema map[string]interface{}) error {

	tmpl, err := template.New("indexTemplate").Parse(indexTemplate)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	params := es.Parameters
	params["ElasticSearchIndex"] = indexName
	properties, err := es.avroToProperties(avroSchema)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	params["ElasticSearchProperties"] = properties

	var indexTemplateBuffer bytes.Buffer
	err = tmpl.Execute(&indexTemplateBuffer, params)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	indexTemplateParsed := indexTemplateBuffer.String()
	indexTemplateParsed = strings.ReplaceAll(indexTemplateParsed, "\n", "")
	indexTemplateParsed = strings.ReplaceAll(indexTemplateParsed, "\t", "")

	req := &esapi.IndicesCreateRequest{
		Index: indexName,
		Body:  strings.NewReader(indexTemplateParsed),
	}

	res, err := req.Do(es.Context, es.Client)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	_, _, err = parseResponse(res)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return nil
}

//avroToProperties transforms an avro schema into elasticsearch mapping properties
func (es GenericElasticsearch) avroToProperties(avroSchema map[string]interface{}) (properties string, err error) {
	if avroSchema["fields"] == nil {
		return properties, errors.Wrap(errors.New("fields missing from avro schema"), 0)
	}

	fields := avroSchema["fields"].([]interface{})
	esProperties, err := parseAvroFields(fields)
	if err != nil {
		return properties, errors.Wrap(err, 0)
	}

	propertiesBytes, err := json.Marshal(esProperties)
	if err != nil {
		return properties, errors.Wrap(err, 0)
	}

	return string(propertiesBytes), nil
}

func parseAvroFields(avroFields []interface{}) (map[string]interface{}, error) {
	esProperties := make(map[string]interface{})
	for _, f := range avroFields {
		avroField := f.(map[string]interface{})
		esProperty := make(map[string]interface{})

		var avroFieldTypeObject map[string]interface{}
		if reflect.TypeOf(avroField["type"]).Kind() == reflect.Slice {
			/* This IF statement handles an array type, e.g.
			"type": [
				"null",
				{
					"type": "string",
					"xjoin.type": "string"
				}
			]
			*/
			avroFieldTypeArray := avroField["type"].([]interface{})
			if reflect.TypeOf(avroFieldTypeArray[0]).Kind() != reflect.String || avroFieldTypeArray[0].(string) != "null" {
				return nil, errors.Wrap(errors.New("avro field's type must be [null, type_object{}]"), 0)
			}

			if reflect.TypeOf(avroFieldTypeArray[1]).Kind() == reflect.Map {
				avroFieldTypeObject = avroFieldTypeArray[1].(map[string]interface{})
			} else {
				return nil, errors.Wrap(errors.New(
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
			return nil, errors.Wrap(
				errors.New(
					fmt.Sprintf(
						"avro field's type must be an object or union of [null, object{}], kind: %s", reflect.TypeOf(avroField["type"]).Kind())), 0)
		}

		esProperty["type"] = avroTypeToElasticsearchType(avroFieldTypeObject["xjoin.type"].(string))

		if esProperty["type"] == "object" {
			if avroFieldTypeObject["xjoin.fields"] != nil {
				nestedProperties, err := parseAvroFields(avroFieldTypeObject["xjoin.fields"].([]interface{}))
				if err != nil {
					return nil, errors.Wrap(err, 0)
				}
				esProperty["properties"] = nestedProperties
			}
		}

		esProperties[avroField["name"].(string)] = esProperty
	}
	return esProperties, nil
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
