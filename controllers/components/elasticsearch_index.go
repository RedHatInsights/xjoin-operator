package components

import (
	"github.com/go-errors/errors"
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

func (es *ElasticsearchIndex) SetName(name string) {
	es.name = strings.ToLower(name)
}

func (es *ElasticsearchIndex) SetVersion(version string) {
	es.version = version
}

func (es *ElasticsearchIndex) Name() string {
	return es.name + "." + es.version
}

func (es *ElasticsearchIndex) Create() (err error) {
	err = es.GenericElasticsearch.CreateIndex(es.Name(), es.Template, es.Properties, es.WithPipeline)
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

func (es *ElasticsearchIndex) CheckDeviation() (problem, err error) {
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
