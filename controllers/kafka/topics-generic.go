package kafka

import (
	"fmt"
	"github.com/go-errors/errors"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

type TopicParameters struct {
	Replicas           int
	Partitions         int
	CleanupPolicy      string
	MinCompactionLagMS string
	RetentionBytes     string
	RetentionMS        string
	MessageBytes       string
	CreationTimeout    int
}

func (kafka *GenericKafka) CreateGenericTopic(topicName string, topicParameters TopicParameters) error {
	topic := &unstructured.Unstructured{}
	topic.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      topicName,
			"namespace": kafka.KafkaNamespace,
			"labels": map[string]interface{}{
				"strimzi.io/cluster": kafka.KafkaCluster,
			},
		},
		"spec": map[string]interface{}{
			"replicas":   topicParameters.Replicas,
			"partitions": topicParameters.Partitions,
			"topicName":  topicName,
			"config": map[string]interface{}{
				"cleanup.policy":        topicParameters.CleanupPolicy,
				"min.compaction.lag.ms": topicParameters.MinCompactionLagMS,
				"retention.bytes":       topicParameters.RetentionBytes,
				"retention.ms":          topicParameters.RetentionMS,
				"max.message.bytes":     topicParameters.MessageBytes,
			},
		},
	}

	topic.SetGroupVersionKind(topicGroupVersionKind)

	err := kafka.Client.Create(kafka.Context, topic)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	log.Info("Waiting for topic to be created.", "topic", topicName)

	//wait for the topic to be created in Kafka (condition.status == ready)
	err = wait.PollImmediate(time.Second, time.Duration(topicParameters.CreationTimeout)*time.Second, func() (bool, error) {
		topics := &unstructured.UnstructuredList{}
		topics.SetGroupVersionKind(topicsGroupVersionKind)

		fields := client.MatchingFields{}
		fields["metadata.name"] = topicName
		fields["metadata.namespace"] = kafka.KafkaNamespace
		labels := client.MatchingLabels{}
		labels["name"] = topicName
		err = kafka.Client.List(kafka.Context, topics, fields)
		if err != nil {
			return false, err
		}

		if len(topics.Items) == 0 {
			return false, nil
		}

		item := topics.Items[0]

		//in tests the status will never be updated
		//this pretends Strimzi created the topic and updated the status
		if kafka.Test == true {
			return true, nil
		}

		if item.Object["status"] == nil {
			return false, nil
		}
		status := item.Object["status"].(map[string]interface{})
		conditions := status["conditions"].([]interface{})
		for _, condition := range conditions {
			conditionMap := condition.(map[string]interface{})
			if conditionMap["type"] == "Ready" {
				if conditionMap["status"] == "True" {
					return true, nil
				} else {
					return false, nil
				}
			}
		}

		return false, nil
	})

	if err != nil {
		return errors.Wrap(errors.New(fmt.Sprintf("timed out waiting for Kafka Topic %s to be created", topicName)), 0)
	}

	return nil
}

func (kafka *GenericKafka) DeleteTopic(topicName string) error {
	if topicName == "" {
		return nil
	}

	topic := &unstructured.Unstructured{}
	topic.SetName(topicName)
	topic.SetNamespace(kafka.KafkaNamespace)
	topic.SetGroupVersionKind(topicGroupVersionKind)

	if err := kafka.Client.Delete(kafka.Context, topic); err != nil && !k8errors.IsNotFound(err) {
		return errors.Wrap(err, 0)
	}

	return nil
}

func (kafka *GenericKafka) ListTopicNamesForPrefix(prefix string) ([]string, error) {
	topics := &unstructured.UnstructuredList{}
	topics.SetGroupVersionKind(topicsGroupVersionKind)

	err := kafka.Client.List(
		kafka.Context, topics, client.InNamespace(kafka.KafkaNamespace))

	var response []string
	if topics.Items != nil {
		for _, topic := range topics.Items {
			if strings.Index(topic.GetName(), prefix) == 0 {
				response = append(response, topic.GetName())
			}
		}
	}

	return response, err
}

func (kafka *GenericKafka) GetTopic(topicName string) (*unstructured.Unstructured, error) {
	topic := &unstructured.Unstructured{}
	topic.SetGroupVersionKind(topicGroupVersionKind)
	err := kafka.Client.Get(
		kafka.Context,
		client.ObjectKey{Name: topicName, Namespace: kafka.KafkaNamespace},
		topic)
	return topic, err
}

func (kafka *GenericKafka) CheckIfTopicExists(name string) (bool, error) {
	if name == "" {
		return false, nil
	}

	if _, err := kafka.GetTopic(name); err != nil && k8errors.IsNotFound(err) {
		return false, nil
	} else if err == nil {
		return true, nil
	} else {
		return false, errors.Wrap(err, 0)
	}
}
