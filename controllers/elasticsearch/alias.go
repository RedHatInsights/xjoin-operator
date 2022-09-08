package elasticsearch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"io/ioutil"
	"strconv"
)

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

func (es *ElasticSearch) UpdateAlias(alias string, version string) error {
	return es.UpdateAliasByFullIndexName(alias, es.ESIndexName(version))
}

func (es *ElasticSearch) GetCurrentIndicesWithAlias(name string) ([]string, error) {
	if name == "" {
		return nil, nil
	}

	req := esapi.CatAliasesRequest{
		Name:   []string{name},
		Format: "JSON",
	}
	ctx, cancel := utils.DefaultContext()
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

func (es *ElasticSearch) AliasName() string {
	if es.parametersMap["ManagedKafka"] == true {
		return "xjoin.inventory.hosts"
	} else {
		return es.resourceNamePrefix + ".hosts"
	}
}
