package kafka

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (kafka *ManagedTopics) TopicName(pipelineVersion string) string {
	return ""
}

func (kafka *ManagedTopics) CreateTopicByFullName(topicName string, dryRun bool) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (kafka *ManagedTopics) CreateTopic(pipelineVersion string, dryRun bool) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (kafka *ManagedTopics) DeleteTopicByPipelineVersion(pipelineVersion string) error {
	return nil
}

func (kafka *ManagedTopics) DeleteAllTopics() error {
	return nil
}

func (kafka *ManagedTopics) ListTopicNamesForPipelineVersion(pipelineVersion string) ([]string, error) {
	return nil, nil
}
