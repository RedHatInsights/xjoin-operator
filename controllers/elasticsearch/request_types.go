package elasticsearch

type UpdateAliasAction interface{}
type UpdateAliasRequest struct {
	Actions UpdateAliasAction `json:"actions,omitempty"`
}

type RemoveAliasAction struct {
	Remove UpdateAliasIndex `json:"remove,omitempty"`
}

type AddAliasAction struct {
	Add UpdateAliasIndex `json:"add,omitempty"`
}

type UpdateAliasIndex struct {
	Index        string `json:"index,omitempty"`
	Alias        string `json:"alias,omitempty"`
	IsWriteIndex bool   `json:"is_write_index,omitempty"`
}
