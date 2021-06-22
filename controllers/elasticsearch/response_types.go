package elasticsearch

import "github.com/redhatinsights/xjoin-operator/controllers/data"

type SearchIDsResponse struct {
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		Hits []struct {
			ID string `json:"_id"`
		} `json:"hits"`
	} `json:"hits"`
	ScrollID string `json:"_scroll_id"`
}

type SearchHostsResponse struct {
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		Hits []struct {
			Host data.Host `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type CatAliasResponse struct {
	Alias string `json:"alias"`
	Index string `json:"index"`
}

type CountIDsResponse struct {
	Count int `json:"count"`
}
