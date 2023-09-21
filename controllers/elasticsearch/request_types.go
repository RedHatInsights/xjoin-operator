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

type QueryHostsById struct {
	Query struct {
		Bool struct {
			Filter struct {
				IDs struct {
					Values []string `json:"values"`
				} `json:"ids"`
			} `json:"filter"`
		} `json:"bool"`
	} `json:"query"`
}

type QueryHostIDsList struct {
	Query struct {
		Bool struct {
			Filter struct {
				IDs struct {
					Values []string `json:"values"`
				} `json:"ids"`
			} `json:"filter"`
		} `json:"bool"`
	} `json:"query"`
}

type QueryHostIDsRange struct {
	Query struct {
		Range struct {
			ModifiedOn struct {
				Lt string `json:"lt"`
				Gt string `json:"gt"`
			} `json:"modified_on"`
		} `json:"range"`
	} `json:"query"`
}

type JsonProcessor struct {
	If    string `json:"if,omitempty"`
	Field string `json:"field,omitempty"`
}

type ScriptProcessor struct {
	Lang   string `json:"lang,omitempty"`
	If     string `json:"if,omitempty"`
	Source string `json:"source,omitempty"`
}

type PipelineProcessor struct {
	Json   *JsonProcessor   `json:"json,omitempty"`
	Script *ScriptProcessor `json:"script,omitempty"`
}

type Pipeline struct {
	Description string              `json:"description,omitempty"`
	Processors  []PipelineProcessor `json:"processors"`
}
