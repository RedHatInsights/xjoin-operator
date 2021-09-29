package components

type KafkaTopic struct {
	name    string
	version string
}

func NewKafkaTopic() *KafkaTopic {
	return &KafkaTopic{}
}

func (kt *KafkaTopic) SetName(name string) {
	kt.name = name
}

func (kt *KafkaTopic) SetVersion(version string) {
	kt.version = version
}

func (kt *KafkaTopic) Name() string {
	return "KafkaTopic"
}

func (kt *KafkaTopic) Create() (err error) {
	return
}

func (kt *KafkaTopic) Delete() (err error) {
	return
}

func (kt *KafkaTopic) CheckDeviation() (err error) {
	return
}

func (kt *KafkaTopic) Exists() (exists bool, err error) {
	return
}

func (kt *KafkaTopic) ListInstalledVersions() (versions []string, err error) {
	return
}
