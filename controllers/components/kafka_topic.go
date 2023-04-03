package components

import (
	"fmt"
	"strings"

	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)


type KafkaTopic struct {
	name            string
	version         string
	KafkaTopics     kafka.StrimziTopics
	TopicParameters kafka.TopicParameters
}

func (kt *KafkaTopic) SetName(name string) {
	kt.name = strings.ToLower(name)
}

func (kt *KafkaTopic) SetVersion(version string) {
	kt.version = version
}

func (kt *KafkaTopic) Name() string {
	return kt.name + "." + kt.version
}

func (kt *KafkaTopic) Create() (err error) {
	err = kt.KafkaTopics.CreateGenericTopic(kt.Name(), kt.TopicParameters)
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
	found, err := kt.Exists()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		return fmt.Errorf("KafkaTopic named, %s, does not exist.", kt.name), nil
	}
	
	// leave this commented line for testing.
	// topicIn, err := kt.KafkaTopics.GetTopic("platform.inventory.events")
	topicIn, err := kt.KafkaTopics.GetTopic(kt.name)

	if err != nil {
		return fmt.Errorf("Error encountered when getting KafkaTopic named, %s", kt.name), errors.Wrap(err, 0)
	}

	if topicIn != nil {
		var allTopics []unstructured.Unstructured
		allTopics, err := kt.KafkaTopics.GetAllTopics()
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		// make the topic data accessible, which is private by design
		topicInPtr, ok := topicIn.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("problem getting the topic %s details", topicIn), nil
		}

		for _, topic := range allTopics {
			if topicInPtr.GetName() == topic.GetName() {
				if equality.Semantic.DeepEqual(*topicInPtr, topic) {
					return nil, nil
				} else { 
					return fmt.Errorf("KafkaTopic named %s has changed.", topic.GetName()), nil
				}
			}
		}
	} else {
		problem = fmt.Errorf("Kafka topic named, \"%s\", not found", kt.name)
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
