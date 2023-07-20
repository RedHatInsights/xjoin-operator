package components

import (
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	"strings"
)

type ElasticsearchIndex struct {
	name                 string
	version              string
	Template             string
	Properties           string
	GenericElasticsearch elasticsearch.GenericElasticsearch
	WithPipeline         bool
	events               events.Events
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
		es.events.Warning("CreateElasticsearchIndexFailure",
			"Unable to create ElasticsearchIndex %s", es.Name())
		return errors.Wrap(err, 0)
	}
	es.events.Normal("CreatedElasticsearchIndex",
		"ElasticsearchIndex %s was successfully created", es.Name())
	return
}

func (es *ElasticsearchIndex) Delete() (err error) {
	err = es.GenericElasticsearch.DeleteIndexByFullName(es.Name())
	if err != nil {
		es.events.Warning("DeleteElasticsearchIndexFailure",
			"Unable to delete ElasticsearchIndex %s", es.Name())
		return errors.Wrap(err, 0)
	}
	es.events.Normal("DeleteElasticsearchIndex",
		"ElasticsearchIndex %s was successfully deleted", es.Name())
	return
}

//CheckDeviation will only compare the mappings portion of the index document.
//This is done because Elasticsearch adds extra settings that are not explicitly defined by the operator
func (es *ElasticsearchIndex) CheckDeviation() (problem, err error) {
	//build the expected index
	expectedIndexString, err := es.GenericElasticsearch.CreateIndex(
		es.Name(), es.Template, es.Properties, es.WithPipeline, true)
	if err != nil {
		es.events.Warning("ElasticsearchIndexCheckDeviationFailed",
			"Unable to create mock ElasticsearchIndex %s", es.Name())
		return nil, errors.Wrap(err, 0)
	}

	var expectedIndex map[string]interface{}
	err = json.Unmarshal([]byte(expectedIndexString), &expectedIndex)
	if err != nil {
		es.events.Warning("ElasticsearchIndexCheckDeviationFailed",
			"Unable to unmarshal expected index string into a map for ElasticsearchIndex %s", es.Name())
		return nil, errors.Wrap(err, 0)
	}
	expectedIndexMappings, ok := expectedIndex["mappings"].(map[string]interface{})
	if !ok {
		es.events.Warning("ElasticsearchIndexCheckDeviationFailed",
			"Unable to unmarshal expected index mappings into a map for ElasticsearchIndex %s", es.Name())
		return nil, errors.Wrap(errors.New("unable to unmarshall expected index mappings"), 0)
	}

	//get the already created (existing) index
	found, err := es.Exists()
	if err != nil {
		es.events.Warning("ElasticsearchIndexCheckDeviationFailed",
			"Unable to check if ElasticsearchIndex %s exists", es.Name())
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		es.events.Warning("ElasticsearchIndexDeviationFound",
			"ElasticsearchIndex %s does not exist", es.Name())
		return fmt.Errorf("the Elasticsearch index, %s, does not exist", es.Name()), nil
	}
	existingIndexString, err := es.GenericElasticsearch.GetIndex(es.Name())
	if err != nil {
		es.events.Warning("ElasticsearchIndexCheckDeviationFailed",
			"Unable to get ElasticsearchIndex %s", es.Name())
		return nil, errors.Wrap(err, 0)
	}
	var existingIndex map[string]interface{}
	err = json.Unmarshal([]byte(existingIndexString), &existingIndex)
	if err != nil {
		es.events.Warning("ElasticsearchIndexCheckDeviationFailed",
			"Unable to unmarshall existing index string into a map for ElasticsearchIndex %s", es.Name())
		return nil, errors.Wrap(err, 0)
	}
	existingIndexSub, ok := existingIndex[es.Name()].(map[string]interface{})
	if !ok {
		es.events.Warning("ElasticsearchIndexCheckDeviationFailed",
			"Unable to unmarshall existing index into a map for ElasticsearchIndex %s", es.Name())
		return nil, errors.Wrap(errors.New("unable to unmarshall existing index"), 0)
	}
	existingIndexMappings, ok := existingIndexSub["mappings"].(map[string]interface{})
	if !ok {
		es.events.Warning("ElasticsearchIndexCheckDeviationFailed",
			"Unable to unmarshall existing index mappings into a map for ElasticsearchIndex %s", es.Name())
		return nil, errors.Wrap(errors.New("unable to unmarshall existing index mappings"), 0)
	}

	//compare the expected and existing indexes
	if existingIndex == nil {
		es.events.Warning("ElasticsearchIndexDeviationFound",
			"Existing ElasticsearchIndex %s was not found", es.Name())
		return fmt.Errorf("the Elasticsearch index named, %s, was not found", es.Name()), nil
	} else {
		diff := cmp.Diff(expectedIndexMappings, existingIndexMappings)

		if len(diff) > 0 {
			es.events.Warning("ElasticsearchIndexDeviationFound",
				"ElasticsearchIndex %s mappings have changed", es.Name())
			return fmt.Errorf("the Elasticsearch index mappings changed: %s", diff), nil
		}
	}

	return
}

func (es *ElasticsearchIndex) Exists() (exists bool, err error) {
	exists, err = es.GenericElasticsearch.IndexExists(es.Name())
	if err != nil {
		es.events.Warning("ElasticsearchIndexExistsFailed",
			"Unable to check if ElasticsearchIndex %s exists", es.Name())
		return false, errors.Wrap(err, 0)
	}
	return
}

func (es *ElasticsearchIndex) ListInstalledVersions() (versions []string, err error) {
	versions, err = es.GenericElasticsearch.ListIndicesForPrefix(es.name)
	if err != nil {
		es.events.Warning("ElasticsearchIndexListInstalledVersionsFailed",
			"Unable to ListIndicesForPrefix for ElasticsearchIndex %s", es.Name())
		return nil, errors.Wrap(err, 0)
	}
	return
}

func (es *ElasticsearchIndex) Reconcile() (err error) {
	return nil
}

func (es *ElasticsearchIndex) SetEvents(e events.Events) {
	es.events = e
}
