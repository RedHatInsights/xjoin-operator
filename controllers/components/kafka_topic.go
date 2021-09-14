package components

type KafkaTopic struct{}

func NewKafkaTopic() *KafkaTopic {
	return &KafkaTopic{}
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
