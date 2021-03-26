package elasticsearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"
)

var log = logger.NewLogger("elasticsearch")

type ElasticSearch struct {
	Client             *elasticsearch.Client
	resourceNamePrefix string
	pipelineTemplate   string
	parametersMap      map[string]interface{}
	indexTemplate      string
}

func NewElasticSearch(
	url string,
	username string,
	password string,
	resourceNamePrefix string,
	pipelineTemplate string,
	indexTemplate string,
	parametersMap map[string]interface{}) (*ElasticSearch, error) {

	es := new(ElasticSearch)
	es.resourceNamePrefix = resourceNamePrefix
	es.pipelineTemplate = pipelineTemplate
	es.parametersMap = parametersMap
	es.indexTemplate = indexTemplate

	cfg := elasticsearch.Config{
		Addresses: []string{url},
		Username:  username,
		Password:  password,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	client, err := elasticsearch.NewClient(cfg)
	es.Client = client
	if err != nil {
		return es, err
	}

	return es, nil
}

func (es *ElasticSearch) SetResourceNamePrefix(updatedPrefix string) {
	es.resourceNamePrefix = updatedPrefix
}

func (es *ElasticSearch) IndexExists(indexName string) (bool, error) {
	res, err := es.Client.Indices.Exists([]string{indexName})
	if err != nil {
		return false, err
	}

	responseCode, _, err := parseResponse(res)
	if err != nil && responseCode != 404 {
		return false, err
	} else if responseCode == 404 {
		return false, nil
	}

	return true, nil
}

func (es *ElasticSearch) CreateIndex(pipelineVersion string) error {
	tmpl, err := template.New("indexTemplate").Parse(es.indexTemplate)
	if err != nil {
		return err
	}

	params := es.parametersMap
	params["ElasticSearchIndex"] = es.ESIndexName(pipelineVersion)
	params["ElasticSearchPipeline"] = es.ESPipelineName(pipelineVersion)

	var indexTemplateBuffer bytes.Buffer
	err = tmpl.Execute(&indexTemplateBuffer, es.parametersMap)
	if err != nil {
		return err
	}
	indexTemplateParsed := indexTemplateBuffer.String()
	indexTemplateParsed = strings.ReplaceAll(indexTemplateParsed, "\n", "")
	indexTemplateParsed = strings.ReplaceAll(indexTemplateParsed, "\t", "")

	req := &esapi.IndicesCreateRequest{
		Index: es.ESIndexName(pipelineVersion),
		Body:  strings.NewReader(indexTemplateParsed),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	res, err := req.Do(ctx, es.Client)

	if err != nil {
		return err
	}

	_, _, err = parseResponse(res)
	return err
}

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
	_, err := es.Client.Ingest.DeletePipeline(esPipeline)
	if err != nil {
		return err
	}
	return nil
}

func (es *ElasticSearch) DeleteIndexByFullName(index string) error {
	res, err := es.Client.Indices.Delete([]string{index})
	if err != nil {
		return err
	}

	responseCode, _, err := parseResponse(res)
	if err != nil && responseCode != 404 {
		return err
	}

	return nil
}

func (es *ElasticSearch) DeleteIndex(version string) error {
	return es.DeleteIndexByFullName(es.ESIndexName(version))
}

func (es *ElasticSearch) UpdateAliasByFullIndexName(alias string, index string) error {

	var req UpdateAliasRequest

	removeIndex := UpdateAliasIndex{
		Index: "*",
		Alias: alias,
	}
	addSourceIndex := UpdateAliasIndex{
		Index: index,
		Alias: alias,
	}
	addSinkIndex := UpdateAliasIndex{
		Index:        index,
		Alias:        alias,
		IsWriteIndex: true,
	}
	removeUpdateAction := RemoveAliasAction{
		Remove: removeIndex,
	}
	addSourceUpdateAction := AddAliasAction{
		Add: addSourceIndex,
	}
	addSinkUpdateAction := AddAliasAction{
		Add: addSinkIndex,
	}

	actions := []UpdateAliasAction{removeUpdateAction, addSourceUpdateAction, addSinkUpdateAction}
	req.Actions = actions

	reqJSON, err := json.Marshal(req)
	res, err := es.Client.Indices.UpdateAliases(bytes.NewReader(reqJSON))

	if err != nil {
		return err
	}

	statusCode, _, err := parseResponse(res)
	if statusCode >= 300 || err != nil {
		return err
	}

	return nil
}

func (es *ElasticSearch) AliasName() string {
	return es.resourceNamePrefix + ".hosts"
}

func (es *ElasticSearch) GetCurrentIndicesWithAlias(name string) ([]string, error) {
	if name == "" {
		return nil, nil
	}

	req := esapi.CatAliasesRequest{
		Name:   []string{name},
		Format: "JSON",
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	res, err := req.Do(ctx, es.Client)
	if err != nil {
		return nil, err
	}

	byteValue, _ := ioutil.ReadAll(res.Body)

	if res.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf(
			"Unable to get current indices with alias. StatusCode: %s, Body: %s",
			strconv.Itoa(res.StatusCode), string(byteValue)))
	}

	var aliasesResponse []CatAliasResponse
	err = json.Unmarshal(byteValue, &aliasesResponse)
	if err != nil {
		return nil, err
	}

	var indices []string
	for _, val := range aliasesResponse {
		indices = append(indices, val.Index)
	}

	return indices, nil
}

func (es *ElasticSearch) ListIndices() ([]string, error) {
	req := esapi.CatIndicesRequest{
		Format: "JSON",
		Index:  []string{es.resourceNamePrefix + ".*"},
		H:      []string{"index"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	res, err := req.Do(ctx, es.Client)
	if err != nil {
		return nil, err
	}

	byteValue, _ := ioutil.ReadAll(res.Body)

	var indicesJSON []map[string]string
	err = json.Unmarshal(byteValue, &indicesJSON)
	if err != nil {
		return nil, err
	}

	var indices []string
	for _, index := range indicesJSON {
		indices = append(indices, index["index"])
	}

	defer res.Body.Close()
	return indices, nil
}

func (es *ElasticSearch) CountIndex(index string) (int, error) {
	req := esapi.CatCountRequest{
		Format: "JSON",
		Index:  []string{index},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	res, err := req.Do(ctx, es.Client)
	if err != nil {
		return -1, err
	}

	byteValue, _ := ioutil.ReadAll(res.Body)

	var countJSON []map[string]interface{}
	err = json.Unmarshal(byteValue, &countJSON)
	if err != nil {
		return -1, err
	}

	response, err := strconv.ParseInt(countJSON[0]["count"].(string), 10, 64)
	if err != nil {
		return -1, err
	}

	return int(response), nil
}

func (es *ElasticSearch) GetHostIDs(index string) ([]string, error) {
	size := new(int)
	*size = 10000
	searchReq := esapi.SearchRequest{
		Index:  []string{index},
		Scroll: time.Duration(1) * time.Minute,
		Query:  "*",
		Source: []string{"id"},
		Size:   size,
		Sort:   []string{"_doc"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	searchRes, err := searchReq.Do(ctx, es.Client)
	if err != nil {
		return nil, err
	}

	ids, searchJSON, err := parseSearchResponse(searchRes)
	if err != nil {
		return nil, err
	}

	if searchJSON.Hits.Total.Value == 0 {
		return ids, nil
	}

	moreHits := true
	scrollID := searchJSON.ScrollID

	for moreHits == true {
		scrollReq := esapi.ScrollRequest{
			Scroll:   time.Duration(1) * time.Minute,
			ScrollID: scrollID,
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()
		scrollRes, err := scrollReq.Do(ctx, es.Client)
		if err != nil {
			return nil, err
		}

		moreIds, scrollJSON, err := parseSearchResponse(scrollRes)
		if err != nil {
			return nil, err
		}
		ids = append(ids, moreIds...)
		scrollID = scrollJSON.ScrollID

		if len(scrollJSON.Hits.Hits) == 0 {
			moreHits = false
		}
	}

	return ids, nil
}

func (es *ElasticSearch) ESIndexName(pipelineVersion string) string {
	return ESIndexName(es.resourceNamePrefix, pipelineVersion)
}

func ESIndexName(resourceNamePrefix string, pipelineVersion string) string {
	return resourceNamePrefix + "." + pipelineVersion
}

func parseSearchResponse(scrollRes *esapi.Response) ([]string, SearchIDsResponse, error) {
	var ids []string
	var searchJSON SearchIDsResponse
	byteValue, _ := ioutil.ReadAll(scrollRes.Body)
	err := json.Unmarshal(byteValue, &searchJSON)
	if err != nil {
		return nil, searchJSON, err
	}

	for _, hit := range searchJSON.Hits.Hits {
		ids = append(ids, hit.ID)
	}

	return ids, searchJSON, nil
}

func parseResponse(res *esapi.Response) (int, map[string]interface{}, error) {
	if res.IsError() {
		_, err := io.Copy(ioutil.Discard, res.Body)
		if err != nil {
			return -1, nil, err
		}
		return res.StatusCode, nil, errors.New(
			fmt.Sprintf("Elasticsearch API error: %s, %s", strconv.Itoa(res.StatusCode), res.Body))
	}

	bodyBytes, err := ioutil.ReadAll(res.Body)
	err = res.Body.Close()
	if err != nil {
		return -1, nil, err
	}

	var bodyMap map[string]interface{}

	if len(bodyBytes) > 0 {
		err = json.Unmarshal(bodyBytes, &bodyMap)
		if err != nil {
			log.Error(err,
				"Unable to parse ES response body to map",
				"body", string(bodyBytes))
			return -1, nil, err
		}
	}

	return res.StatusCode, bodyMap, nil
}
