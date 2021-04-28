package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/redhatinsights/xjoin-operator/controllers/data"
	"io/ioutil"
	"time"
)

func (es *ElasticSearch) GetHostsByIds(index string, hostIds []string) ([]data.Host, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	var query QueryHostsById
	query.Query.IDs.Values = hostIds
	reqJSON, err := json.Marshal(query)
	requestSize := len(hostIds)

	searchReq := esapi.SearchRequest{
		Index: []string{index},
		//Source: []string{"id"},
		Size: &requestSize,
		Sort: []string{"_id"},
		Body: bytes.NewReader(reqJSON),
	}

	searchRes, err := searchReq.Do(ctx, es.Client)
	if err != nil {
		return nil, err
	}

	hosts, err := parseSearchHostsResponse(searchRes)
	if err != nil {
		return nil, err
	}

	return hosts, nil
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	searchRes, err := searchReq.Do(ctx, es.Client)
	if err != nil {
		return nil, err
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

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
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
