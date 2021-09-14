package components

type DataSourcePipeline struct{}

func NewDataSourcePipeline() *DataSourcePipeline {
	return &DataSourcePipeline{}
}

func (as *DataSourcePipeline) Name() string {
	return "DataSourcePipeline"
}

func (as *DataSourcePipeline) Create() (err error) {
	return
}

func (as *DataSourcePipeline) Delete() (err error) {
	return
}

func (as *DataSourcePipeline) CheckDeviation() (err error) {
	return
}

func (as *DataSourcePipeline) Exists() (exists bool, err error) {
	return
}
