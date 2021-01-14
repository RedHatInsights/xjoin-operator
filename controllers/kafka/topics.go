package kafka

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	return fmt.Sprintf(kafka.Parameters.ResourceNamePrefix.String() + "." + pipelineVersion)
}

func (kafka *Kafka) CreateTopic(pipelineVersion string) error {
	topic := &unstructured.Unstructured{}
	topic.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      kafka.TopicName(pipelineVersion),
			"namespace": kafka.Namespace,
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

	if kafka.Owner != nil {
		if err := controllerutil.SetControllerReference(kafka.Owner, topic, kafka.OwnerScheme); err != nil {
			return err
		}

		labels := topic.GetLabels()
		labels["xjoin/owner"] = string(kafka.Owner.GetUID())
		topic.SetLabels(labels)
	}

	return kafka.Client.Create(context.TODO(), topic)
}

func (kafka *Kafka) DeleteTopic(topicName string) error {
	topic := &unstructured.Unstructured{}
	topic.SetName(topicName)
	topic.SetNamespace(kafka.Namespace)
	topic.SetGroupVersionKind(topicGroupVersionKind)

	if err := kafka.Client.Delete(context.TODO(), topic); err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (kafka *Kafka) ListTopicNames() ([]string, error) {
	topics := &unstructured.UnstructuredList{}
	topics.SetGroupVersionKind(topicsGroupVersionKind)

	err := kafka.Client.List(
		context.TODO(), topics, client.InNamespace(kafka.Namespace))

	var response []string
	if topics.Items != nil {
		for _, topic := range topics.Items {
			response = append(response, topic.GetName())
		}
	}

	return response, err
}
