package components

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"strings"

	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type KafkaTopic struct {
	name            string
	version         string
	KafkaTopics     kafka.StrimziTopics
	TopicParameters kafka.TopicParameters
}

func (kt *KafkaTopic) SetName(kind string, name string) {
	kt.name = strings.ToLower(kind + "." + name)
}

func (kt *KafkaTopic) SetVersion(version string) {
	kt.version = version
}

func (kt *KafkaTopic) Name() string {
	return kt.name + "." + kt.version
}

func (kt *KafkaTopic) Create() (err error) {
	_, err = kt.KafkaTopics.CreateGenericTopic(kt.Name(), kt.TopicParameters, false)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (kt *KafkaTopic) Delete() (err error) {
	err = kt.KafkaTopics.DeleteTopic(kt.Name())
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (kt *KafkaTopic) CheckDeviation() (problem, err error) {
	//build the expected topic
	expectedTopic, err := kt.KafkaTopics.CreateGenericTopic(kt.Name(), kt.TopicParameters, true)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	//get the already created (existing) topic
	found, err := kt.Exists()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		return fmt.Errorf("the KafkaTopic named, %s, does not exist", kt.Name()), nil
	}

	existingTopic, err := kt.KafkaTopics.GetTopic(kt.Name())
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if existingTopic != nil {
		existingTopicPtr, existingTopicOk := existingTopic.(*unstructured.Unstructured)
		if !existingTopicOk {
			return nil, errors.Wrap(errors.New(
				"existingTopic is the wrong type in KafkaTopic.CheckDeviation"), 0)
		}

		expectedTopicUnstructured := expectedTopic.UnstructuredContent()
		existingTopicUnstructured := existingTopicPtr.UnstructuredContent()

		specDiff := cmp.Diff(
			expectedTopicUnstructured["spec"].(map[string]interface{}),
			existingTopicUnstructured["spec"].(map[string]interface{}),
			utils.NumberNormalizer)

		if len(specDiff) > 0 {
			return fmt.Errorf("topic spec has changed: %s", specDiff), nil
		}

		if existingTopicPtr.GetNamespace() != expectedTopic.GetNamespace() {
			return fmt.Errorf(
				"topic namespace has changed from: %s to %s",
				existingTopicPtr.GetNamespace(),
				expectedTopic.GetNamespace()), nil
		}
	} else {
		problem = fmt.Errorf("the Kafka topic named, %s, not found", kt.Name())
	}
	return
}

func (kt *KafkaTopic) Exists() (exists bool, err error) {
	exists, err = kt.KafkaTopics.CheckIfTopicExists(kt.Name())
	if err != nil {
		return false, errors.Wrap(err, 0)
	}
	return
}

func (kt *KafkaTopic) ListInstalledVersions() (versions []string, err error) {
	topicNames, err := kt.KafkaTopics.ListTopicNamesForPrefix(kt.name)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	for _, name := range topicNames {
		versions = append(versions, strings.Split(name, kt.name+".")[1])
	}
	return
}

func (kt *KafkaTopic) Reconcile() (err error) {
	return nil
}
