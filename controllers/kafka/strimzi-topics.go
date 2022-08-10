package kafka

import (
	"fmt"
	"github.com/go-errors/errors"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
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

func (t *StrimziTopics) TopicName(pipelineVersion string) string {
	return fmt.Sprintf(t.ResourceNamePrefix + "." + pipelineVersion + ".public.hosts")
}

func (t *StrimziTopics) createTopicByFullName(topicName string, dryRun bool) (*unstructured.Unstructured, error) {
	topic := &unstructured.Unstructured{}
	topic.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      topicName,
			"namespace": t.KafkaClusterNamespace,
			"labels": map[string]interface{}{
				"strimzi.io/cluster":   t.KafkaCluster,
				"resource.name.prefix": t.ResourceNamePrefix,
			},
		},
		"spec": map[string]interface{}{
			"replicas":   t.TopicParameters.Replicas,
			"partitions": t.TopicParameters.Partitions,
			"topicName":  topicName,
			"config": map[string]interface{}{
				"cleanup.policy":        t.TopicParameters.CleanupPolicy,
				"min.compaction.lag.ms": t.TopicParameters.MinCompactionLagMS,
				"retention.bytes":       t.TopicParameters.RetentionBytes,
				"retention.ms":          t.TopicParameters.RetentionMS,
				"max.message.bytes":     t.TopicParameters.MessageBytes,
			},
		},
	}

	topic.SetGroupVersionKind(topicGroupVersionKind)

	if dryRun {
		return topic, nil
	}

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err := t.Client.Create(ctx, topic)
	if err != nil {
		return nil, err
	}

	if !t.Test {
		log.Info("Waiting for topic to be created.", "topic", topicName)

		//wait for the topic to be created in Kafka (condition.status == ready)
		err = wait.PollImmediate(time.Second, time.Duration(t.TopicParameters.CreationTimeout)*time.Second, func() (bool, error) {
			topics := &unstructured.UnstructuredList{}
			topics.SetGroupVersionKind(topicsGroupVersionKind)

			fields := client.MatchingFields{}
			fields["metadata.name"] = topicName
			labels := client.MatchingLabels{}
			labels["name"] = topicName
			err = t.Client.List(ctx, topics, fields)
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

func (t *StrimziTopics) CreateTopic(pipelineVersion string, dryRun bool) error {
	_, err := t.createTopicByFullName(t.TopicName(pipelineVersion), dryRun)
	return err
}

func (t *StrimziTopics) DeleteTopicByPipelineVersion(pipelineVersion string) error {
	err := t.DeleteTopic(t.TopicName(pipelineVersion))
	return err
}

//DeleteAllTopics is only used in tests
func (t *StrimziTopics) DeleteAllTopics() error {
	ctx, cancel := utils.DefaultContext()
	defer cancel()

	topic := &unstructured.Unstructured{}
	topic.SetNamespace(t.KafkaClusterNamespace)
	topic.SetGroupVersionKind(topicsGroupVersionKind)

	err := t.Client.DeleteAllOf(
		ctx,
		topic,
		client.InNamespace(t.KafkaClusterNamespace),
		client.MatchingLabels{"resource.name.prefix": t.ResourceNamePrefix},
		client.GracePeriodSeconds(0))
	if err != nil {
		return err
	}

	log.Info("Waiting for topics to be deleted")
	err = wait.PollImmediate(time.Second, time.Duration(300)*time.Second, func() (bool, error) {
		topics, err := t.ListTopicNamesForPrefix(t.ResourceNamePrefix)
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

//ListTopicNamesForPipelineVersion is only used in tests
func (t *StrimziTopics) ListTopicNamesForPipelineVersion(pipelineVersion string) ([]string, error) {
	topics := &unstructured.UnstructuredList{}
	topics.SetGroupVersionKind(topicsGroupVersionKind)

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err := t.Client.List(
		ctx, topics, client.InNamespace(t.KafkaClusterNamespace))

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

func (t *StrimziTopics) CheckDeviation(pipelineVersion string) (problem error, err error) {
	topicName := t.TopicName(pipelineVersion)
	topic, err := t.GetTopic(topicName)
	topicObj := topic.(*unstructured.Unstructured)
	if err != nil || topic == nil {
		if k8errors.IsNotFound(err) {
			return fmt.Errorf(
				"topic %s not found in %s",
				topicName, t.KafkaClusterNamespace), nil
		}
		return nil, err
	}

	if topicObj.GetLabels()[LabelStrimziCluster] != t.KafkaCluster {
		return fmt.Errorf(
			"KafkaCluster changed from %s to %s",
			topicObj.GetLabels()[LabelStrimziCluster],
			t.KafkaCluster), nil
	}

	newTopic, err := t.createTopicByFullName(t.TopicName(pipelineVersion), true)
	if err != nil {
		return nil, err
	}

	topicUnstructured := topicObj.UnstructuredContent()
	newTopicUnstructured := newTopic.UnstructuredContent()

	specDiff := cmp.Diff(
		topicUnstructured["spec"].(map[string]interface{}),
		newTopicUnstructured["spec"].(map[string]interface{}),
		utils.NumberNormalizer)

	if len(specDiff) > 0 {
		return fmt.Errorf("topic spec has changed: %s", specDiff), nil
	}

	if topicObj.GetNamespace() != newTopic.GetNamespace() {
		return fmt.Errorf(
			"topic namespace has changed from: %s to %s",
			topicObj.GetNamespace(),
			newTopic.GetNamespace()), nil
	}

	return nil, nil
}

func (t *StrimziTopics) CreateGenericTopic(topicName string, topicParameters TopicParameters) error {
	topic := &unstructured.Unstructured{}
	topic.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      topicName,
			"namespace": t.KafkaClusterNamespace,
			"labels": map[string]interface{}{
				"strimzi.io/cluster": t.KafkaCluster,
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

	err := t.Client.Create(t.Context, topic)
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
		fields["metadata.namespace"] = t.KafkaClusterNamespace
		labels := client.MatchingLabels{}
		labels["name"] = topicName
		err = t.Client.List(t.Context, topics, fields)
		if err != nil {
			return false, err
		}

		if len(topics.Items) == 0 {
			return false, nil
		}

		item := topics.Items[0]

		//in tests the status will never be updated
		//this pretends Strimzi created the topic and updated the status
		if t.Test == true {
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

func (t *StrimziTopics) DeleteTopic(topicName string) error {
	if topicName == "" {
		return nil
	}

	topic := &unstructured.Unstructured{}
	topic.SetName(topicName)
	topic.SetNamespace(t.KafkaClusterNamespace)
	topic.SetGroupVersionKind(topicGroupVersionKind)

	if err := t.Client.Delete(t.Context, topic); err != nil && !k8errors.IsNotFound(err) {
		return errors.Wrap(err, 0)
	}

	return nil
}

func (t *StrimziTopics) ListTopicNamesForPrefix(prefix string) ([]string, error) {
	topics := &unstructured.UnstructuredList{}
	topics.SetGroupVersionKind(topicsGroupVersionKind)

	err := t.Client.List(
		t.Context, topics, client.InNamespace(t.KafkaClusterNamespace))

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

func (t *StrimziTopics) GetTopic(topicName string) (interface{}, error) {
	topic := &unstructured.Unstructured{}
	topic.SetGroupVersionKind(topicGroupVersionKind)
	err := t.Client.Get(
		t.Context,
		client.ObjectKey{Name: topicName, Namespace: t.KafkaClusterNamespace},
		topic)
	return topic, err
}

func (t *StrimziTopics) CheckIfTopicExists(name string) (bool, error) {
	if name == "" {
		return false, nil
	}

	if _, err := t.GetTopic(name); err != nil && k8errors.IsNotFound(err) {
		return false, nil
	} else if err == nil {
		return true, nil
	} else {
		return false, errors.Wrap(err, 0)
	}
}
