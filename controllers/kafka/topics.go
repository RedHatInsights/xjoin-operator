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

func TopicName(pipelineVersion string) string {
	return fmt.Sprintf("xjoin.inventory.%s.public.hosts", pipelineVersion)
}

func (kafka *Kafka) CreateTopic(pipelineVersion string) error {
	topic := &unstructured.Unstructured{}
	topic.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      TopicName(pipelineVersion),
			"namespace": kafka.Namespace,
			"labels": map[string]interface{}{
				"strimzi.io/cluster": kafka.KafkaCluster,
			},
		},
		"spec": map[string]interface{}{
			"replicas":   1,
			"partitions": 1,
			"topicName":  TopicName(pipelineVersion),
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
		context.TODO(), topics, client.InNamespace(kafka.Namespace), client.MatchingLabels{LabelOwner: kafka.Owner.GetUIDString()})

	var response []string
	if topics.Items != nil {
		for _, topic := range topics.Items {
			response = append(response, topic.GetName())
		}
	}

	return response, err
}