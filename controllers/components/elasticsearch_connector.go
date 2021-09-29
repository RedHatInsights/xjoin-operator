package components

type ElasticsearchConnector struct {
	name    string
	version string
}

func NewElasticsearchConnector() *ElasticsearchConnector {
	return &ElasticsearchConnector{}
}

func (es *ElasticsearchConnector) SetName(name string) {
	es.name = name
}

func (es *ElasticsearchConnector) SetVersion(version string) {
	es.version = version
}

func (es *ElasticsearchConnector) Name() string {
	return "ElasticsearchConnector"
}

func (es *ElasticsearchConnector) Create() (err error) {
	return
}

func (es *ElasticsearchConnector) Delete() (err error) {
	return
}

func (es *ElasticsearchConnector) CheckDeviation() (err error) {
	return
}

func (es *ElasticsearchConnector) Exists() (exists bool, err error) {
	return
}

func (es *ElasticsearchConnector) ListInstalledVersions() (versions []string, err error) {
	return
}
