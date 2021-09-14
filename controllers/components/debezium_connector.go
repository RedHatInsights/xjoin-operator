package components

type DebeziumConnector struct {
}

func NewDebeziumConnector() *DebeziumConnector {
	return &DebeziumConnector{}
}

func (dc *DebeziumConnector) Name() string {
	return "DebeziumConnector"
}

func (dc *DebeziumConnector) Create() (err error) {
	return
}

func (dc *DebeziumConnector) Delete() (err error) {
	return
}

func (dc *DebeziumConnector) CheckDeviation() (err error) {
	return
}

func (dc *DebeziumConnector) Exists() (exists bool, err error) {
	return
}
