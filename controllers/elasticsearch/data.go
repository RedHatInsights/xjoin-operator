package elasticsearch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/redhatinsights/xjoin-operator/controllers/data"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"io/ioutil"
	"sort"
	"strings"
	"time"
)

func (es *ElasticSearch) GetHostsByIds(index string, hostIds []string) ([]data.Host, error) {
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	var query QueryHostsById
	query.Query.Bool.Filter.IDs.Values = hostIds
	reqJSON, err := json.Marshal(query)
	requestSize := len(hostIds)

	searchReq := esapi.SearchRequest{
		Index: []string{index},
		Size:  &requestSize,
		Sort:  []string{"_id"},
		Body:  bytes.NewReader(reqJSON),
	}

	searchRes, err := searchReq.Do(ctx, es.Client)
	if err != nil {
		return nil, err
	}
	if searchRes.StatusCode >= 400 {
		bodyBytes, _ := ioutil.ReadAll(searchRes.Body)

		return nil, errors.New(fmt.Sprintf(
			"invalid response code when getting hosts by id. StatusCode: %v, Body: %s",
			searchRes.StatusCode, bodyBytes))
	}

	hosts, err := parseSearchHostsResponse(searchRes)
	if err != nil {
		return nil, err
	}

	hosts = sortHostFields(hosts)

	return hosts, nil
}

func sortHostFields(hosts []data.Host) []data.Host {
	for i, host := range hosts {
		//java's encoder (used in the flattenlist SMT) doesn't encode * but the golang encoder does
		for k := range hosts[i].TagsString {
			hosts[i].TagsString[k] = strings.ReplaceAll(hosts[i].TagsString[k], "*", "%2A")
		}

		data.OrderedBy(data.NamespaceComparator, data.KeyComparator, data.ValueComparator).Sort(host.TagsStructured)
		sort.Strings(host.TagsSearch)
		sort.Strings(host.TagsString)
	}

	return hosts
}

func (es *ElasticSearch) getHostIDsQuery(index string, reqJSON []byte) ([]string, error) {
	size := new(int)
	*size = 10000

	searchReq := esapi.SearchRequest{
		Index:  []string{index},
		Scroll: time.Duration(1) * time.Minute,
		Body:   bytes.NewReader(reqJSON),
		Source: []string{"id"},
		Size:   size,
		Sort:   []string{"_doc"},
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	searchRes, err := searchReq.Do(ctx, es.Client)
	if err != nil {
		return nil, err
	}

	if searchRes.StatusCode >= 400 {
		bodyBytes, _ := ioutil.ReadAll(searchRes.Body)

		return nil, errors.New(fmt.Sprintf(
			"invalid response code when getting hosts ids. StatusCode: %v, Body: %s",
			searchRes.StatusCode, bodyBytes))
	}

	ids, searchJSON, err := parseSearchIdsResponse(searchRes)
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

		ctx, cancel := utils.DefaultContext()
		defer cancel()
		scrollRes, err := scrollReq.Do(ctx, es.Client)
		if err != nil {
			return nil, err
		}

		moreIds, scrollJSON, err := parseSearchIdsResponse(scrollRes)
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

func (es *ElasticSearch) GetHostIDsByIdList(index string, ids []string) ([]string, error) {
	var query QueryHostIDsList
	query.Query.Bool.Filter.IDs.Values = ids
	reqJSON, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	return es.getHostIDsQuery(index, reqJSON)
}

func (es *ElasticSearch) GetHostIDsByModifiedOn(index string, start time.Time, end time.Time) ([]string, error) {
	var query QueryHostIDsRange
	query.Query.Range.ModifiedOn.Lt = end.Format(utils.TimeFormat())
	query.Query.Range.ModifiedOn.Gt = start.Format(utils.TimeFormat())
	reqJSON, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	log.Info("ElasticSearch.GetHostIDsQuery", "body", query)

	return es.getHostIDsQuery(index, reqJSON)
}

func parseSearchHostsResponse(res *esapi.Response) ([]data.Host, error) {
	var hosts []data.Host
	var searchHostsJSON SearchHostsResponse
	byteValue, _ := ioutil.ReadAll(res.Body)
	err := json.Unmarshal(byteValue, &searchHostsJSON)
	if err != nil {
		return nil, err
	}

	for _, hit := range searchHostsJSON.Hits.Hits {
		hosts = append(hosts, hit.Host)
	}

	return hosts, nil
}

func parseSearchIdsResponse(scrollRes *esapi.Response) ([]string, SearchIDsResponse, error) {
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
