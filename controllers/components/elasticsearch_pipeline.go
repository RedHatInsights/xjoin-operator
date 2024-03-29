package components

import (
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"strings"
)

type ElasticsearchPipeline struct {
	name                 string
	version              string
	JsonFields           []string
	GenericElasticsearch elasticsearch.GenericElasticsearch
	events               events.Events
	log                  logger.Log
}

func (es *ElasticsearchPipeline) SetLogger(log logger.Log) {
	es.log = log
}

func (es *ElasticsearchPipeline) SetName(kind string, name string) {
	es.name = strings.ToLower(kind + "." + name)
}

func (es *ElasticsearchPipeline) SetVersion(version string) {
	es.version = version
}

func (es *ElasticsearchPipeline) Name() string {
	return es.name + "." + es.version
}

func (es *ElasticsearchPipeline) Create() (err error) {
	pipeline, err := es.jsonFieldsToESPipeline()
	if err != nil {
		es.events.Warning("CreateElasticsearchPipelineFailed",
			"Unable to convert JSON fields to ElasticsearchPipeline for %s", es.Name())
		return errors.Wrap(err, 0)
	}
	err = es.GenericElasticsearch.CreatePipeline(es.Name(), pipeline)
	if err != nil {
		es.events.Warning("CreateElasticsearchPipelineFailed",
			"Unable to create ElasticsearchPipeline %s", es.Name())
		return errors.Wrap(err, 0)
	}

	es.events.Normal("CreatedElasticsearchPipeline",
		"ElasticsearchPipeline %s was successfully created", es.Name())
	return
}

func (es *ElasticsearchPipeline) Delete() (err error) {
	err = es.GenericElasticsearch.DeletePipeline(es.Name())
	if err != nil {
		es.events.Warning("DeleteElasticsearchPipelineFailed",
			"Unable to delete ElasticsearchPipeline %s", es.Name())
		return errors.Wrap(err, 0)
	}
	es.events.Normal("DeleteElasticsearchPipeline",
		"ElasticsearchPipeline %s was successfully deleted", es.Name())
	return
}

func (es *ElasticsearchPipeline) CheckDeviation() (problem, err error) {
	//TODO implement elasticsearchpipeline checkdeviation
	return
}

func (es *ElasticsearchPipeline) Exists() (exists bool, err error) {
	exists, err = es.GenericElasticsearch.PipelineExists(es.Name())
	if err != nil {
		es.events.Warning("ElasticsearchPipelineExistsFailed",
			"Unable to check if ElasticsearchPipeline %s exists", es.Name())
		return false, errors.Wrap(err, 0)
	}
	return
}

func (es *ElasticsearchPipeline) ListInstalledVersions() (versions []string, err error) {
	pipelines, err := es.GenericElasticsearch.ListPipelinesForPrefix(es.name)
	if err != nil {
		es.events.Warning("ElasticsearchPipelineListInstalledVersionsFailed",
			"Unable to ListPipelinesForPrefix for ElasticsearchPipeline %s", es.Name())
		return nil, errors.Wrap(err, 0)
	}

	for _, pipeline := range pipelines {
		versions = append(versions, strings.Split(pipeline, es.name+".")[1])
	}
	return
}

func (es *ElasticsearchPipeline) Reconcile() (err error) {
	return nil
}

func (es *ElasticsearchPipeline) jsonFieldsToESPipeline() (pipeline string, err error) {
	var pipelineObj elasticsearch.Pipeline
	pipelineObj.Description = "test"
	pipelineObj.Processors = make([]elasticsearch.PipelineProcessor, 0)

	var timestampProcessor elasticsearch.PipelineProcessor
	timestampScriptProcessor := elasticsearch.ScriptProcessor{
		Lang:   "painless",
		Source: "ctx.__es_write_ms = System.currentTimeMillis();",
	}
	timestampProcessor.Script = &timestampScriptProcessor
	pipelineObj.Processors = append(pipelineObj.Processors, timestampProcessor)

	for _, jsonField := range es.JsonFields {
		var processor elasticsearch.PipelineProcessor
		jsonProcessor := elasticsearch.JsonProcessor{
			Field: jsonField,
			If:    fmt.Sprintf("ctx.%s != null", jsonField),
		}
		processor.Json = &jsonProcessor
		pipelineObj.Processors = append(pipelineObj.Processors, processor)
	}

	pipelineJson, err := json.Marshal(pipelineObj)
	if err != nil {
		return pipeline, errors.Wrap(err, 0)
	}
	return string(pipelineJson), nil
}

func (es *ElasticsearchPipeline) SetEvents(e events.Events) {
	es.events = e
}
