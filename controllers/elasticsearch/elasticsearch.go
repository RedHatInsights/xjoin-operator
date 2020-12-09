package elasticsearch

import (
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
	"strings"
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

func (es *ElasticSearch) AddAliasToIndex(alias string, index string) error {
	res, err := es.client.Indices.UpdateAliases(strings.NewReader(
		fmt.Sprintf(`{"actions": [{"add": {"index": "%s", "alias": "%s"}}]}`, alias, index)))

	if err != nil {
		return err
	}

	return parseResponse(res)
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
