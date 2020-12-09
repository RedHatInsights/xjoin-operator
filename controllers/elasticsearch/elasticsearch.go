package elasticsearch

import (
	"crypto/tls"
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

func (es *ElasticSearch) Init() (err error) {
	cfg := elasticsearch.Config{
		Addresses: []string{
			"https://elasticsearch:9200",
		},
		Username: "xjoin",
		Password: "xjoin1337",
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	es.client, err = elasticsearch.NewClient(cfg)
	if err != nil {
		return err
	}

	return nil
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

func (es *ElasticSearch) CreateIndex(indexName string) error {
	res, err := es.client.Indices.Create(indexName)
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

func parseResponse(res *esapi.Response) error {
	io.Copy(ioutil.Discard, res.Body)
	defer res.Body.Close()

	if res.IsError() {
		return errors.New(
			fmt.Sprintf("Elasticsearch API error: %s, %s", strconv.Itoa(res.StatusCode), res.Body))
	}

	return nil
}
