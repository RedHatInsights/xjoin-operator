package kafka

// response types
type ManagedTopicResponse struct {
	Kind  string             `json:"kind,omitempty"`
	Items []ManagedTopicItem `json:"items"`
}

type ManagedTopicPartitionIsr struct {
	Id int `json:"id,omitempty"`
}

type ManagedTopicPartitionLeader struct {
	Id int `json:"id,omitempty"`
}

type ManagedTopicPartitionReplica struct {
	Id int `json:"id,omitempty"`
}

type ManagedTopicPartition struct {
	Replicas  []ManagedTopicPartitionReplica `json:"replicas"`
	Isr       []ManagedTopicPartitionIsr     `json:"isr"`
	Leader    ManagedTopicPartitionLeader    `json:"leader,omitempty"`
	Id        int                            `json:"id,omitempty"`
	Partition int                            `json:"partition,omitempty"`
}

type ManagedTopicItem struct {
	Id         string                  `json:"id,omitempty"`
	Kind       string                  `json:"kind,omitempty"`
	Href       string                  `json:"href,omitempty"`
	Name       string                  `json:"name,omitempty"`
	IsInternal bool                    `json:"isInternal,omitempty"`
	Partitions []ManagedTopicPartition `json:"partitions,omitempty"`
	Config     []ManagedTopicConfig    `json:"config,omitempty"`
}

// request types
type ManagedTopicRequest struct {
	Name     string               `json:"name,omitempty"`
	Settings ManagedTopicSettings `json:"settings,omitempty"`
}

type ManagedTopicConfig struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type ManagedTopicSettings struct {
	NumPartitions int                  `json:"numPartitions,omitempty"`
	Replicas      int                  `json:"numReplicas,omitempty"`
	Config        []ManagedTopicConfig `json:"config,omitempty"`
}
