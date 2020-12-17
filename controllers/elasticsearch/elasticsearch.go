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
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type ElasticSearch struct {
	client *elasticsearch.Client
}

func NewElasticSearch(url string, username string, password string) (*ElasticSearch, error) {
	es := new(ElasticSearch)
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
	es.client = client
	if err != nil {
		return es, err
	}

	return es, nil
}

func (es *ElasticSearch) IndexExists(indexName string) (bool, error) {
	res, err := es.client.Indices.Exists([]string{indexName})
	if err != nil {
		return false, err
	}

	if res.StatusCode == 404 {
		return false, nil
	}

	err = parseResponse(res)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *ElasticSearch) CreateIndex(pipelineVersion string) error {
	res, err := es.client.Indices.Create(ESIndexName(pipelineVersion))
	if err != nil {
		return err
	}
	return parseResponse(res)
}

func (es *ElasticSearch) DeleteIndex(indexName string) error {
	res, err := es.client.Indices.Delete([]string{indexName})
	if err != nil {
		return err
	}
	return parseResponse(res)
}

func (es *ElasticSearch) UpdateAlias(index string) error {

	var req UpdateAliasRequest

	removeIndex := UpdateAliasIndex{
		Index: "*",
		Alias: "xjoin.inventory.hosts",
	}
	addSourceIndex := UpdateAliasIndex{
		Index: index,
		Alias: "xjoin.inventory.hosts",
	}
	addSinkIndex := UpdateAliasIndex{
		Index:        index,
		Alias:        "xjoin.inventory.hosts",
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
	res, err := es.client.Indices.UpdateAliases(bytes.NewReader(reqJSON))

	if err != nil {
		return err
	}

	return parseResponse(res)
}

func (es *ElasticSearch) GetCurrentIndicesWithAlias() ([]string, error) {
	req := esapi.CatAliasesRequest{
		Name:   []string{"xjoin.inventory.hosts"},
		Format: "JSON",
	}
	res, err := req.Do(context.Background(), es.client)
	if err != nil {
		return nil, err
	}

	byteValue, _ := ioutil.ReadAll(res.Body)

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
		Index:  []string{"xjoin.inventory.hosts.*"},
		H:      []string{"index"},
	}
	res, err := req.Do(context.Background(), es.client)
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

func (es *ElasticSearch) CountIndex(index string) (int64, error) {
	req := esapi.CatCountRequest{
		Format: "JSON",
		Index:  []string{index},
	}

	res, err := req.Do(context.Background(), es.client)
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

	return response, nil
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

	searchRes, err := searchReq.Do(context.Background(), es.client)
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

		scrollRes, err := scrollReq.Do(context.Background(), es.client)
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

func parseSearchResponse(scrollRes *esapi.Response) ([]string, SearchIDsResponse, error) {
	var ids []string
	var searchJSON SearchIDsResponse
	byteValue, _ := ioutil.ReadAll(scrollRes.Body)
	err := json.Unmarshal(byteValue, &searchJSON)
	if err != nil {
		return nil, searchJSON, err
	}

	//TODO: more performant way to do this? Some built in golang function?
	for _, hit := range searchJSON.Hits.Hits {
		ids = append(ids, hit.ID)
	}

	return ids, searchJSON, nil
}

func ESIndexName(pipelineVersion string) string {
	return "xjoin.inventory.hosts." + pipelineVersion
}

func parseResponse(res *esapi.Response) error {
	_, err := io.Copy(ioutil.Discard, res.Body)
	if err != nil {
		return err
	}
	//TODO: handle error here
	defer res.Body.Close()

	if res.IsError() {
		return errors.New(
			fmt.Sprintf("Elasticsearch API error: %s, %s", strconv.Itoa(res.StatusCode), res.Body))
	}

	return nil
}
