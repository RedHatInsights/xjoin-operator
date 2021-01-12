package elasticsearch

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

type CatAliasResponse struct {
	Alias string `json:"alias"`
	Index string `json:"index"`
}
