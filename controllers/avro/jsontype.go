package avro

type IndexField struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	Ref  bool   `json:"xjoinref,omitempty"`
}

type IndexSchema struct {
	Type   string       `json:"type,omitempty"`
	Name   string       `json:"name,omitempty"`
	Fields []IndexField `json:"fields,omitempty"`
}

type SourceSchema struct {
	Type        string        `json:"type,omitempty"`
	Name        string        `json:"name,omitempty"`
	Namespace   string        `json:"namespace,omitempty"`
	Fields      []interface{} `json:"fields,omitempty"`
	ConnectName string        `json:"connect.name,omitempty"`
}
