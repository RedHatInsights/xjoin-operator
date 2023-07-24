package components

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
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
	events          events.Events
	log             logger.Log
}

func (kt *KafkaTopic) SetLogger(log logger.Log) {
	kt.log = log
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
		kt.events.Warning("CreateKafkaTopicFailed",
			"Unable to create KafkaTopic %s", kt.Name())
		return errors.Wrap(err, 0)
	}

	kt.events.Normal("CreatedKafkaTopic",
		"KafkaTopic %s was successfully created", kt.Name())
	return
}

func (kt *KafkaTopic) Delete() (err error) {
	err = kt.KafkaTopics.DeleteTopic(kt.Name())
	if err != nil {
		kt.events.Warning("DeleteKafkaTopicFailed",
			"Unable to delete KafkaTopic %s", kt.Name())
		return errors.Wrap(err, 0)
	}
	kt.events.Normal("DeletedKafkaTopic",
		"KafkaTopic %s was successfully deleted", kt.Name())
	return
}

func (kt *KafkaTopic) CheckDeviation() (problem, err error) {
	//build the expected topic
	expectedTopic, err := kt.KafkaTopics.CreateGenericTopic(kt.Name(), kt.TopicParameters, true)
	if err != nil {
		kt.events.Warning("KafkaTopicCheckDeviationFailed",
			"Unable to create expected KafkaTopic %s", kt.Name())
		return nil, errors.Wrap(err, 0)
	}

	//get the already created (existing) topic
	found, err := kt.Exists()
	if err != nil {
		kt.events.Warning("KafkaTopicCheckDeviationFailed",
			"Unable to check if KafkaTopic %s exists", kt.Name())
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		kt.events.Warning("KafkaTopicDeviationFound",
			"KafkaTopic %s does not exist", kt.Name())
		return fmt.Errorf("the KafkaTopic named, %s, does not exist", kt.Name()), nil
	}

	existingTopic, err := kt.KafkaTopics.GetTopic(kt.Name())
	if err != nil {
		kt.events.Warning("KafkaTopicCheckDeviationFailed",
			"Unable to get KafkaTopic %s", kt.Name())
		return nil, errors.Wrap(err, 0)
	}

	if existingTopic != nil {
		existingTopicPtr, existingTopicOk := existingTopic.(*unstructured.Unstructured)
		if !existingTopicOk {
			kt.events.Warning("KafkaTopicCheckDeviationFailed",
				"Unable to parse existing KafkaTopic %s", kt.Name())
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
			kt.events.Warning("KafkaTopicDeviationFound",
				"KafkaTopic %s spec has changed", kt.Name())
			return fmt.Errorf("topic spec has changed: %s", specDiff), nil
		}

		if existingTopicPtr.GetNamespace() != expectedTopic.GetNamespace() {
			kt.events.Warning("KafkaTopicDeviationFound",
				"KafkaTopic %s namespace has changed", kt.Name())
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
		kt.events.Warning("KafkaTopicExistsFailed",
			"Unable to CheckIfTopicExists for KafkaTopic %s", kt.Name())
		return false, errors.Wrap(err, 0)
	}
	return
}

func (kt *KafkaTopic) ListInstalledVersions() (versions []string, err error) {
	topicNames, err := kt.KafkaTopics.ListTopicNamesForPrefix(kt.name)
	if err != nil {
		kt.events.Warning("KafkaTopicListInstalledVersionsFailed",
			"Unable to ListInstalledVersions for KafkaTopic %s", kt.Name())
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

func (kt *KafkaTopic) SetEvents(e events.Events) {
	kt.events = e
}
