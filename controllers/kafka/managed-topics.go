package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"golang.org/x/oauth2/clientcredentials"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

const jsonContentType = "application/json"

func NewManagedTopics(options ManagedTopicsOptions) *ManagedTopics {
	credentialsConfig := clientcredentials.Config{
		ClientID:     options.ClientId,
		ClientSecret: options.ClientSecret,
		TokenURL:     options.TokenURL,
		Scopes:       []string{"openid api.iam.service_accounts"},
	}
	managedTopics := ManagedTopics{
		Options: options,
		client:  credentialsConfig.Client(context.Background()),
		baseurl: options.AdminURL + "/api/v1/topics",
	}
	return &managedTopics
}

func (t *ManagedTopics) TopicName(pipelineVersion string) string {
	return fmt.Sprintf(t.Options.ResourceNamePrefix + "." + pipelineVersion + ".public.hosts")
}

func (t *ManagedTopics) CreateTopicByName(topicName string) error {
	body := ManagedTopicRequest{
		Name: topicName,
		Settings: ManagedTopicSettings{
			NumPartitions: t.Options.TopicParameters.Partitions,
			Replicas:      t.Options.TopicParameters.Replicas,
			Config: []ManagedTopicConfig{{
				Key:   "retention.ms",
				Value: t.Options.TopicParameters.RetentionMS,
			}, {
				Key:   "retention.bytes",
				Value: t.Options.TopicParameters.RetentionBytes,
			}, {
				Key:   "cleanup.policy",
				Value: t.Options.TopicParameters.CleanupPolicy,
			}, {
				Key:   "min.compaction.lag.ms",
				Value: t.Options.TopicParameters.MinCompactionLagMS,
			}},
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	log.Info("Managed Kafka Create Topic Body: " + string(bodyBytes))

	res, err := t.client.Post(t.baseurl, jsonContentType, bytes.NewReader(bodyBytes))
	if err != nil {
		return errors.Wrap(err, 0)
	}

	_, _, err = parseResponse(res)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}

func (t *ManagedTopics) CreateTopic(pipelineVersion string, dryRun bool) error {
	return t.CreateTopicByName(t.TopicName(pipelineVersion))
}

func (t *ManagedTopics) CheckDeviation(pipelineVersion string) (problem error, err error) {
	topic, err := t.GetTopic(t.TopicName(pipelineVersion))
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	existingTopic := topic.(ManagedTopicItem)

	var existingTopicRetentionMs string
	var existingTopicRetentionBytes string
	var existingTopicCleanupPolicy string
	var existingTopicMinCompactionLagMS string

	for _, config := range existingTopic.Config {
		switch config.Key {
		case "retention.ms":
			existingTopicRetentionMs = config.Value
		case "retention.bytes":
			existingTopicRetentionBytes = config.Value
		case "cleanup.policy":
			existingTopicCleanupPolicy = config.Value
		case "min.compaction.lag.ms":
			existingTopicMinCompactionLagMS = config.Value
		}
	}

	existingTopicReplicas := len(existingTopic.Partitions[0].Replicas)

	existingTopicSettings := ManagedTopicSettings{
		NumPartitions: len(existingTopic.Partitions),
		Replicas:      existingTopicReplicas,
		Config: []ManagedTopicConfig{{
			Key:   "retention.ms",
			Value: existingTopicRetentionMs,
		}, {
			Key:   "retention.bytes",
			Value: existingTopicRetentionBytes,
		}, {
			Key:   "cleanup.policy",
			Value: existingTopicCleanupPolicy,
		}, {
			Key:   "min.compaction.lag.ms",
			Value: existingTopicMinCompactionLagMS,
		}}}

	newTopicSettings := ManagedTopicSettings{
		NumPartitions: t.Options.TopicParameters.Partitions,
		Replicas:      existingTopicReplicas, //TODO hack to skip checking replicas because managed kafka automatically sets replicas to 3
		Config: []ManagedTopicConfig{{
			Key:   "retention.ms",
			Value: t.Options.TopicParameters.RetentionMS,
		}, {
			Key:   "retention.bytes",
			Value: t.Options.TopicParameters.RetentionBytes,
		}, {
			Key:   "cleanup.policy",
			Value: t.Options.TopicParameters.CleanupPolicy,
		}, {
			Key:   "min.compaction.lag.ms",
			Value: t.Options.TopicParameters.MinCompactionLagMS,
		}}}

	topicDiff := cmp.Diff(existingTopicSettings, newTopicSettings, utils.NumberNormalizer)

	if len(topicDiff) > 0 {
		return fmt.Errorf("topic settings changed: %s", topicDiff), nil
	}

	return nil, nil
}

func (t *ManagedTopics) ListTopics() ([]byte, error) {
	res, err := t.client.Get(t.baseurl + "?size=100&page=1&filter=ckyrouac")
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	_, bodyBytes, err := parseResponse(res)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return bodyBytes, nil
}

func (t *ManagedTopics) ListTopicNamesForPrefix(prefix string) ([]string, error) {
	res, err := t.client.Get(t.baseurl + "?size=100&page=1&filter=" + prefix)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	_, bodyBytes, err := parseResponse(res)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	var body ManagedTopicResponse
	if len(bodyBytes) > 0 {
		err = json.Unmarshal(bodyBytes, &body)
		if err != nil {
			err = errors.Wrap(err, 0)
			log.Error(err,
				"Unable to parse Managed Kafka response body to map",
				"body", string(bodyBytes))
			return nil, err
		}
	}

	if body.Kind != "TopicList" {
		return nil, errors.Wrap(errors.New("Invalid Kind ("+body.Kind+")in response from Managed Kafka API when listing topics"), 0)
	}

	var response []string
	for _, topic := range body.Items {
		if strings.Index(topic.Name, prefix) == 0 {
			response = append(response, topic.Name)
		}
	}

	log.Info("ListTopicNamesForPrefix response: " + strings.Join(response, ","))

	return response, nil
}

func (t *ManagedTopics) DeleteTopicByPipelineVersion(pipelineVersion string) error {
	err := t.DeleteTopic(t.TopicName(pipelineVersion))
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return nil
}

func (t *ManagedTopics) DeleteTopic(topicName string) error {
	log.Info("Deleting topic: " + topicName)
	req, err := http.NewRequest("DELETE", t.baseurl+"/"+topicName, nil)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	res, err := t.client.Do(req)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	_, _, err = parseResponse(res)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return nil
}

func (t *ManagedTopics) GetTopic(topicName string) (interface{}, error) {
	res, err := t.client.Get(t.baseurl + "/" + topicName)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	_, bodyBytes, err := parseResponse(res)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	var body ManagedTopicItem
	if len(bodyBytes) > 0 {
		err = json.Unmarshal(bodyBytes, &body)
		if err != nil {
			err = errors.Wrap(err, 0)
			log.Error(err,
				"Unable to parse Managed Kafka response body to map",
				"body", string(bodyBytes))
			return nil, err
		}
	}
	return body, nil
}

// DeleteAllTopics is only used for tests, a stub will do for now
func (t *ManagedTopics) DeleteAllTopics() error {
	return nil
}

// ListTopicNamesForPipelineVersion is only used for tests, a stub will do for now
func (t *ManagedTopics) ListTopicNamesForPipelineVersion(pipelineVersion string) ([]string, error) {
	return nil, nil
}

func parseResponse(res *http.Response) (int, []byte, error) {
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		bodyBytes, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return res.StatusCode, nil, errors.Wrap(err, 0)
		}
		return res.StatusCode, nil, errors.Wrap(errors.New(
			fmt.Sprintf("Manged Kafka API error: %s, %s", strconv.Itoa(res.StatusCode), string(bodyBytes))), 0)
	}

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return res.StatusCode, nil, errors.Wrap(err, 0)
	}

	return res.StatusCode, bodyBytes, nil
}
