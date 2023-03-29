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
	name := kt.name
	client := kt.KafkaTopics  // kt.KafkaTopics perform CRUD operations.

	// leave this commented line for testing.
	// topicIn, err := client.GetTopic("platform.inventory.events")
	topicIn, err := client.GetTopic(name)

	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if topicIn != nil {
		var allTopics []unstructured.Unstructured
		allTopics, err := client.GetAllTopics()
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		// make the topic data accessible, which is private by design
		topicInPtr, ok := topicIn.(*unstructured.Unstructured)
		if !ok {
			problem = fmt.Errorf("problem getting the topic %s details", topicIn)
		}

		// de-reference the topic pointer for comparison
		topicInClear := *topicInPtr
		fmt.Printf("Input topic name: %s\n", topicInClear.GetName())

		if len(allTopics) > 0 {
			for _, topic := range allTopics {

				if topicInClear.GetName() == topic.GetName() {
					if equality.Semantic.DeepEqual(topicInClear, topic) {
						fmt.Printf("\nTopic named %s is identicle!!!", topic.GetName())
						return nil, nil
					} else { 
						fmt.Printf("\nTopic named %s NOT identicle!!!", topic.GetName())
						problem = fmt.Errorf("KafkaTopic named %s has changed.", topic.GetName())
					}
				}
			}
		}
	} else {
		problem = fmt.Errorf("Kafka topic named, \"%s\", not found", name)
	}
	return
}

func assertNoError(err error) {
	panic("unimplemented")
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
