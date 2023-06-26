package components

import (
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"strings"
)

type ElasticsearchIndex struct {
	name                 string
	version              string
	Template             string
	Properties           string
	GenericElasticsearch elasticsearch.GenericElasticsearch
	WithPipeline         bool
}

func (es *ElasticsearchIndex) SetName(kind string, name string) {
	es.name = strings.ToLower(kind + "." + name)
}

func (es *ElasticsearchIndex) SetVersion(version string) {
	es.version = version
}

func (es *ElasticsearchIndex) Name() string {
	return es.name + "." + es.version
}

func (es *ElasticsearchIndex) Create() (err error) {
	_, err = es.GenericElasticsearch.CreateIndex(es.Name(), es.Template, es.Properties, es.WithPipeline, false)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (es *ElasticsearchIndex) Delete() (err error) {
	err = es.GenericElasticsearch.DeleteIndexByFullName(es.Name())
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

//CheckDeviation will only compare the mappings portion of the index document.
//This is done because Elasticsearch adds extra settings that are not explicitly defined by the operator
func (es *ElasticsearchIndex) CheckDeviation() (problem, err error) {
	//build the expected index
	expectedIndexString, err := es.GenericElasticsearch.CreateIndex(
		es.Name(), es.Template, es.Properties, es.WithPipeline, true)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	var expectedIndex map[string]interface{}
	err = json.Unmarshal([]byte(expectedIndexString), &expectedIndex)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	expectedIndexMappings, ok := expectedIndex["mappings"].(map[string]interface{})
	if !ok {
		return nil, errors.Wrap(errors.New("unable to unmarshall expected index mappings"), 0)
	}

	//get the already created (existing) index
	found, err := es.Exists()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		return fmt.Errorf("the Elasticsearch index, %s, does not exist", es.Name()), nil
	}
	existingIndexString, err := es.GenericElasticsearch.GetIndex(es.Name())
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	var existingIndex map[string]interface{}
	err = json.Unmarshal([]byte(existingIndexString), &existingIndex)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	existingIndexSub, ok := existingIndex[es.Name()].(map[string]interface{})
	if !ok {
		return nil, errors.Wrap(errors.New("unable to unmarshall existing index"), 0)
	}
	existingIndexMappings, ok := existingIndexSub["mappings"].(map[string]interface{})
	if !ok {
		return nil, errors.Wrap(errors.New("unable to unmarshall existing index mappings"), 0)
	}

	//compare the expected and existing indexes
	if existingIndex == nil {
		return fmt.Errorf("the Elasticsearch index named, %s, was not found", es.Name()), nil
	} else {
		diff := cmp.Diff(expectedIndexMappings, existingIndexMappings)

		if len(diff) > 0 {
			return fmt.Errorf("the Elasticsearch index mappings changed: %s", diff), nil
		}
	}

	return
}

func (es *ElasticsearchIndex) Exists() (exists bool, err error) {
	exists, err = es.GenericElasticsearch.IndexExists(es.Name())
	if err != nil {
		return false, errors.Wrap(err, 0)
	}
	return
}

func (es *ElasticsearchIndex) ListInstalledVersions() (versions []string, err error) {
	versions, err = es.GenericElasticsearch.ListIndicesForPrefix(es.name)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return
}

func (es *ElasticsearchIndex) Reconcile() (err error) {
	return nil
}
