package kafka

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

var topicGroupVersionKind = schema.GroupVersionKind{
	Group:   "kafka.strimzi.io",
	Kind:    "KafkaTopic",
	Version: "v1alpha1",
}

var topicsGroupVersionKind = schema.GroupVersionKind{
	Group:   "kafka.strimzi.io",
	Kind:    "KafkaTopicList",
	Version: "v1alpha1",
}

func (kafka *Kafka) TopicName(pipelineVersion string) string {
	return fmt.Sprintf(kafka.Parameters.ResourceNamePrefix.String() + "." + pipelineVersion + ".public.hosts")
}

func (kafka *Kafka) CreateTopic(pipelineVersion string, dryRun bool) (*unstructured.Unstructured, error) {
	topic := &unstructured.Unstructured{}
	topic.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      kafka.TopicName(pipelineVersion),
			"namespace": kafka.Parameters.KafkaClusterNamespace.String(),
			"labels": map[string]interface{}{
				"strimzi.io/cluster": kafka.Parameters.KafkaCluster.String(),
			},
		},
		"spec": map[string]interface{}{
			"replicas":   kafka.Parameters.KafkaTopicReplicas.Int(),
			"partitions": kafka.Parameters.KafkaTopicPartitions.Int(),
			"topicName":  kafka.TopicName(pipelineVersion),
		},
	}

	topic.SetGroupVersionKind(topicGroupVersionKind)

	if dryRun {
		return topic, nil
	}

	return topic, kafka.Client.Create(context.TODO(), topic)
}

func (kafka *Kafka) DeleteTopicByPipelineVersion(pipelineVersion string) error {
	err := kafka.DeleteTopic(kafka.TopicName(pipelineVersion))
	return err
}

func (kafka *Kafka) DeleteTopic(topicName string) error {
	topic := &unstructured.Unstructured{}
	topic.SetName(topicName)
	topic.SetNamespace(kafka.Parameters.KafkaClusterNamespace.String())
	topic.SetGroupVersionKind(topicGroupVersionKind)

	if err := kafka.Client.Delete(context.TODO(), topic); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (kafka *Kafka) ListTopicNamesForPrefix(resourceNamePrefix string) ([]string, error) {
	topics := &unstructured.UnstructuredList{}
	topics.SetGroupVersionKind(topicsGroupVersionKind)

	err := kafka.Client.List(
		context.TODO(), topics, client.InNamespace(kafka.Parameters.KafkaClusterNamespace.String()))

	var response []string
	if topics.Items != nil {
		for _, topic := range topics.Items {
			if strings.Index(topic.GetName(), resourceNamePrefix) == 0 {
				response = append(response, topic.GetName())
			}
		}
	}

	return response, err
}

func (kafka *Kafka) ListTopicNamesForPipelineVersion(pipelineVersion string) ([]string, error) {
	topics := &unstructured.UnstructuredList{}
	topics.SetGroupVersionKind(topicsGroupVersionKind)

	err := kafka.Client.List(
		context.TODO(), topics, client.InNamespace(kafka.Parameters.KafkaClusterNamespace.String()))

	var response []string
	if topics.Items != nil {
		for _, topic := range topics.Items {
			if strings.Index(topic.GetName(), pipelineVersion) != -1 {
				response = append(response, topic.GetName())
			}
		}
	}

	return response, err
}

func (kafka *Kafka) GetTopic(topicName string) (*unstructured.Unstructured, error) {
	topic := &unstructured.Unstructured{}
	topic.SetGroupVersionKind(topicGroupVersionKind)
	err := kafka.Client.Get(
		context.TODO(),
		client.ObjectKey{Name: topicName, Namespace: kafka.Parameters.KafkaClusterNamespace.String()},
		topic)
	return topic, err
}
