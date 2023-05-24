package components

import (
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	"strings"
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
	return //TODO
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
