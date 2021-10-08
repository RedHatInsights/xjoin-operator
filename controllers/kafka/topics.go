package kafka

import (
	"errors"
	"fmt"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

var topicGroupVersionKind = schema.GroupVersionKind{
	Group:   "kafka.strimzi.io",
	Kind:    "KafkaTopic",
	Version: "v1beta2",
}

var topicsGroupVersionKind = schema.GroupVersionKind{
	Group:   "kafka.strimzi.io",
	Kind:    "KafkaTopic",
	Version: "v1beta2",
}

func (kafka *Kafka) TopicName(pipelineVersion string) string {
	return fmt.Sprintf(kafka.Parameters.ResourceNamePrefix.String() + "." + pipelineVersion + ".public.hosts")
}

func (kafka *Kafka) CreateTopicByFullName(topicName string, dryRun bool) (*unstructured.Unstructured, error) {
	topic := &unstructured.Unstructured{}
	topic.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      topicName,
			"namespace": kafka.Parameters.KafkaClusterNamespace.String(),
			"labels": map[string]interface{}{
				"strimzi.io/cluster":   kafka.Parameters.KafkaCluster.String(),
				"resource.name.prefix": kafka.Parameters.ResourceNamePrefix.String(),
			},
		},
		"spec": map[string]interface{}{
			"replicas":   kafka.Parameters.KafkaTopicReplicas.Int(),
			"partitions": kafka.Parameters.KafkaTopicPartitions.Int(),
			"topicName":  topicName,
			"config": map[string]interface{}{
				"cleanup.policy":        kafka.Parameters.KafkaTopicCleanupPolicy.String(),
				"min.compaction.lag.ms": kafka.Parameters.KafkaTopicMinCompactionLagMS.String(),
				"retention.bytes":       kafka.Parameters.KafkaTopicRetentionBytes.String(),
				"retention.ms":          kafka.Parameters.KafkaTopicRetentionMS.String(),
				"max.message.bytes":     kafka.Parameters.KafkaTopicMessageBytes.String(),
			},
		},
	}

	topic.SetGroupVersionKind(topicGroupVersionKind)

	if dryRun {
		return topic, nil
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err := kafka.Client.Create(ctx, topic)
	if err != nil {
		return nil, err
	}

	if !kafka.Test {
		log.Info("Waiting for topic to be created.", "topic", topicName)

		//wait for the topic to be created in Kafka (condition.status == ready)
		err = wait.PollImmediate(time.Second, time.Duration(kafka.Parameters.KafkaTopicCreationTimeout.Int())*time.Second, func() (bool, error) {
			topics := &unstructured.UnstructuredList{}
			topics.SetGroupVersionKind(topicsGroupVersionKind)

			fields := client.MatchingFields{}
			fields["metadata.name"] = topicName
			labels := client.MatchingLabels{}
			labels["name"] = topicName
			err = kafka.Client.List(ctx, topics, fields)
			if err != nil {
				return false, err
			}

			if len(topics.Items) == 0 {
				return false, nil
			}

			item := topics.Items[0]
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
			return nil, errors.New(fmt.Sprintf("timed out waiting for Kafka Topic %s to be created", topicName))
		}
	}

	return topic, nil
}

func (kafka *Kafka) CreateTopic(pipelineVersion string, dryRun bool) (*unstructured.Unstructured, error) {
	return kafka.CreateTopicByFullName(kafka.TopicName(pipelineVersion), dryRun)
}

func (kafka *Kafka) DeleteTopicByPipelineVersion(pipelineVersion string) error {
	err := kafka.DeleteTopic(kafka.TopicName(pipelineVersion))
	return err
}

func (kafka *Kafka) DeleteAllTopics() error {
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	topic := &unstructured.Unstructured{}
	topic.SetNamespace(kafka.Parameters.KafkaClusterNamespace.String())
	topic.SetGroupVersionKind(topicsGroupVersionKind)

	err := kafka.Client.DeleteAllOf(
		ctx,
		topic,
		client.InNamespace(kafka.Parameters.KafkaClusterNamespace.String()),
		client.MatchingLabels{"resource.name.prefix": kafka.Parameters.ResourceNamePrefix.String()},
		client.GracePeriodSeconds(0))
	if err != nil {
		return err
	}

	log.Info("Waiting for topics to be deleted")
	err = wait.PollImmediate(time.Second, time.Duration(300)*time.Second, func() (bool, error) {
		topics, err := kafka.ListTopicNamesForPrefix(kafka.Parameters.ResourceNamePrefix.String())
		if err != nil {
			return false, err
		}
		if len(topics) > 0 {
			return false, nil
		} else {
			return true, nil
		}
	})
	if err != nil {
		return err
	}

	return nil
}

func (kafka *Kafka) ListTopicNamesForPipelineVersion(pipelineVersion string) ([]string, error) {
	topics := &unstructured.UnstructuredList{}
	topics.SetGroupVersionKind(topicsGroupVersionKind)

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err := kafka.Client.List(
		ctx, topics, client.InNamespace(kafka.Parameters.KafkaClusterNamespace.String()))

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
