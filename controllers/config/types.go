package config

type XJoinConfiguration struct {
	ResourceNamePrefix             string
	ConnectCluster                 string
	KafkaCluster                   string
	ElasticSearchURL               string
	ElasticSearchUsername          string
	ElasticSearchPassword          string
	ConfigMapVersion               string
	DebeziumConnectorTemplate      string
	ElasticSearchConnectorTemplate string
	StandardInterval               int64

	ValidationConfig     ValidationConfiguration
	ValidationConfigInit ValidationConfiguration

	ConnectorMaxAge    int64
	ConnectorTasksMax  int64
	ConnectorBatchSize int64
}

type DBParams struct {
	Name     string
	Host     string
	Port     string
	User     string
	Password string
}

type ValidationConfiguration struct {
	Interval            int64
	AttemptsThreshold   int64
	PercentageThreshold int64
}
