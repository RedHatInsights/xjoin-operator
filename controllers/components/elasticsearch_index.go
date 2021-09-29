package components

type ElasticsearchIndex struct {
	name    string
	version string
}

func NewElasticsearchIndex() *ElasticsearchIndex {
	return &ElasticsearchIndex{}
}

func (es *ElasticsearchIndex) SetName(name string) {
	es.name = name
}

func (es *ElasticsearchIndex) SetVersion(version string) {
	es.version = version
}

func (es *ElasticsearchIndex) Name() string {
	return "ElasticsearchIndex"
}

func (es *ElasticsearchIndex) Create() (err error) {
	return
}

func (es *ElasticsearchIndex) Delete() (err error) {
	return
}

func (es *ElasticsearchIndex) CheckDeviation() (err error) {
	return
}

func (es *ElasticsearchIndex) Exists() (exists bool, err error) {
	return
}

func (es *ElasticsearchIndex) ListInstalledVersions() (versions []string, err error) {
	return
}
