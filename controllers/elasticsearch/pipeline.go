package elasticsearch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"strconv"
	"strings"
	"text/template"
	"time"
)

func (es *ElasticSearch) ESPipelineName(pipelineVersion string) string {
	return es.resourceNamePrefix + "." + pipelineVersion
}

func (es *ElasticSearch) ESPipelineExists(pipelineVersion string) (bool, error) {
	req := esapi.IngestGetPipelineRequest{
		DocumentID: es.ESPipelineName(pipelineVersion),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	res, err := req.Do(ctx, es.Client)
	if err != nil {
		return nil, err
	}
	resCode, body, err := parseResponse(res)

	if resCode != 200 {
		return nil, errors.New(fmt.Sprintf(
			"invalid response from ElasticSearch. StatusCode: %s, Body: %s",
			strconv.Itoa(res.StatusCode), body))
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
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
		return nil, errors.New(fmt.Sprintf(
			"Unable to list es pipelines. StatusCode: %s, Body: %s",
			strconv.Itoa(res.StatusCode), body))
	}

	for esPipelineName, _ := range body {
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
