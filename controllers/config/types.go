package config

type XJoinConfiguration struct {
	ConnectCluster                 string
	ElasticSearchURL               string
	ElasticSearchUsername          string
	ElasticSearchPassword          string
	ConfigMapVersion               string
	DebeziumConnectorTemplate      string
	ElasticSearchConnectorTemplate string
	StandardInterval               int64
}

type DBParams struct {
	Name     string
	Host     string
	Port     string
	User     string
	Password string
}
