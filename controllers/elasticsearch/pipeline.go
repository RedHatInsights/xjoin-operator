package elasticsearch

import (
	"bytes"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"strconv"
	"strings"
	"text/template"
)

func (es *ElasticSearch) ESPipelineName(pipelineVersion string) string {
	return es.resourceNamePrefix + "." + pipelineVersion
}

func (es *ElasticSearch) ESPipelineExists(pipelineVersion string) (bool, error) {
	req := esapi.IngestGetPipelineRequest{
		DocumentID: es.ESPipelineName(pipelineVersion),
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	res, err := req.Do(ctx, es.Client)
	if err != nil {
		return false, err
	}

	resCode, _, err := parseResponse(res)
	if resCode == 404 {
		return false, nil
	} else if err != nil {
		return false, err
	} else {
		return true, nil
	}
}

func (es *ElasticSearch) GetESPipeline(pipelineVersion string) (map[string]interface{}, error) {
	req := esapi.IngestGetPipelineRequest{
		DocumentID: es.ESPipelineName(pipelineVersion),
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	res, err := req.Do(ctx, es.Client)
	if err != nil {
		return nil, err
	}
	resCode, body, err := parseResponse(res)

	if resCode != 200 {
		return nil, fmt.Errorf(
			"invalid response from ElasticSearch. StatusCode: %s, Body: %s",
			strconv.Itoa(res.StatusCode), body)
	}

	return body, err
}

func (es *ElasticSearch) CreateESPipeline(pipelineVersion string) error {
	tmpl, err := template.New("pipelineTemplate").Parse(es.pipelineTemplate)
	if err != nil {
		return err
	}

	var pipelineTemplateBuffer bytes.Buffer
	err = tmpl.Execute(&pipelineTemplateBuffer, es.parametersMap)
	if err != nil {
		return err
	}
	pipelineTemplateParsed := pipelineTemplateBuffer.String()
	pipelineTemplateParsed = strings.ReplaceAll(pipelineTemplateParsed, "\n", "")
	pipelineTemplateParsed = strings.ReplaceAll(pipelineTemplateParsed, "\t", "")

	res, err := es.Client.Ingest.PutPipeline(
		es.ESPipelineName(pipelineVersion), strings.NewReader(pipelineTemplateParsed))
	if err != nil {
		return err
	}

	statusCode, _, err := parseResponse(res)
	if err != nil || statusCode != 200 {
		return err
	}

	return nil
}

func (es *ElasticSearch) ListESPipelines(pipelineIds ...string) ([]string, error) {
	req := esapi.IngestGetPipelineRequest{}

	if pipelineIds != nil {
		req.DocumentID = "*" + pipelineIds[0] + "*"
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	res, err := req.Do(ctx, es.Client)
	if err != nil {
		return nil, err
	}
	resCode, body, err := parseResponse(res)
	var esPipelines []string

	if resCode == 404 {
		return esPipelines, nil
	} else if resCode != 200 {
		return nil, fmt.Errorf(
			"Unable to list es pipelines. StatusCode: %s, Body: %s",
			strconv.Itoa(res.StatusCode), body)
	}

	for esPipelineName := range body {
		esPipelines = append(esPipelines, esPipelineName)
	}

	return esPipelines, err
}

func (es *ElasticSearch) DeleteESPipelineByVersion(version string) error {
	return es.DeleteESPipelineByFullName(es.ESPipelineName(version))
}

func (es *ElasticSearch) DeleteESPipelineByFullName(esPipeline string) error {
	if esPipeline == "" {
		return nil
	}

	_, err := es.Client.Ingest.DeletePipeline(esPipeline)
	if err != nil {
		return err
	}
	return nil
}
