package elasticsearch

import (
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
				InsecureSkipVerify: false,
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
