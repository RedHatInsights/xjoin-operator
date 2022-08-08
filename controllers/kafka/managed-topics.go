package kafka

func (t *ManagedTopics) TopicName(pipelineVersion string) string {
	return ""
}

func (t *ManagedTopics) CreateTopic(pipelineVersion string, dryRun bool) error {
	return nil
}

func (t *ManagedTopics) DeleteTopicByPipelineVersion(pipelineVersion string) error {
	return nil
}

func (t *ManagedTopics) DeleteAllTopics() error {
	return nil
}

func (t *ManagedTopics) ListTopicNamesForPipelineVersion(pipelineVersion string) ([]string, error) {
	return nil, nil
}

func (t *ManagedTopics) CheckDeviation(pipelineVersion string) (problem error, err error) {
	return nil, nil
}
