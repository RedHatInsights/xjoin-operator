package elasticsearch

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/go-errors/errors"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
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
	defer res.Body.Close()
	if res.IsError() {
		bodyBytes, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return -1, nil, errors.Wrap(err, 0)
		}
		return res.StatusCode, nil, errors.Wrap(errors.New(
			fmt.Sprintf("Elasticsearch API error: %s, %s", strconv.Itoa(res.StatusCode), string(bodyBytes))), 0)
	}

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return -1, nil, errors.Wrap(err, 0)
	}

	var bodyMap map[string]interface{}

	if len(bodyBytes) > 0 {
		err = json.Unmarshal(bodyBytes, &bodyMap)
		if err != nil {
			err = errors.Wrap(err, 0)
			log.Error(err,
				"Unable to parse ES response body to map",
				"body", string(bodyBytes))
			return -1, nil, err
		}
	}

	return res.StatusCode, bodyMap, nil
}
