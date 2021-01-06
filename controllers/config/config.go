package config

import (
	"fmt"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

const (
	reconcileInterval             = "standard.interval"
	validationInterval            = "validation.interval"
	validationAttemptsThreshold   = "validation.attempts.threshold"
	validationPercentageThreshold = "validation.percentage.threshold"
	defaultResourceNamePrefix     = "xjoin.inventory.hosts"
)

// These keys are excluded when computing a ConfigMap hash.
// Therefore, if they change that won't trigger a pipeline refresh
var keysIgnoredByRefresh []string

func BuildXJoinConfig(instance *xjoin.XJoinPipeline, cm *corev1.ConfigMap) (*XJoinConfiguration, error) {
	var err error
	config := &XJoinConfiguration{}

	if instance != nil && instance.Spec.ResourceNamePrefix != nil {
		config.ResourceNamePrefix = *instance.Spec.ResourceNamePrefix
	} else {
		config.ResourceNamePrefix = getStringValue(cm, "resourceNamePrefix", defaultResourceNamePrefix)
	}

	if instance != nil && instance.Spec.ConnectCluster != nil {
		config.ConnectCluster = *instance.Spec.ConnectCluster
	} else {
		config.ConnectCluster = getStringValue(cm, "connect.cluster", defaultConnectCluster)
	}

	if instance != nil && instance.Spec.KafkaCluster != nil {
		config.KafkaCluster = *instance.Spec.KafkaCluster
	} else {
		config.KafkaCluster = getStringValue(cm, "kafka.cluster", defaultKafkaCluster)
	}

	//TODO: handle a secret for ES params
	config.ElasticSearchURL = getStringValue(cm, "elasticsearch.url", defaultElasticSearchURL)
	config.ElasticSearchUsername = getStringValue(cm, "elasticsearch.url", defaultElasticSearchUsername)
	config.ElasticSearchPassword = getStringValue(cm, "elasticsearch.url", defaultElasticSearchPassword)

	config.DebeziumConnectorTemplate = getStringValue(
		cm, "debezium.connector.config", defaultDebeziumConnectorTemplate)
	config.ElasticSearchConnectorTemplate = getStringValue(
		cm, "elasticsearch.connector.config", defaultElasticSearchConnectorTemplate)

	if config.ConnectorTasksMax, err = getIntValue(cm, "connector.tasks.max", defaultConnectorTasksMax); err != nil {
		return config, err
	}

	if config.ConnectorBatchSize, err = getIntValue(cm, "connector.batch.size", defaultConnectorBatchSize); err != nil {
		return config, err
	}

	if instance != nil && instance.Spec.MaxAge != nil {
		config.ConnectorMaxAge = *instance.Spec.MaxAge
	} else if config.ConnectorMaxAge, err = getIntValue(cm, "connector.max.age", defaultConnectorMaxAge); err != nil {
		return config, err
	}

	if config.StandardInterval, err = getIntValue(cm, reconcileInterval, defaultStandardInterval); err != nil {
		return config, err
	}

	if config.ValidationConfig, err = getValidationConfig(instance, cm, "", defaultValidationConfig); err != nil {
		return config, err
	}

	if config.ValidationConfigInit, err = getValidationConfig(instance, cm, "init.", defaultValidationConfigInit); err != nil {
		return config, err
	}

	config.ConfigMapVersion = utils.ConfigMapHash(cm, keysIgnoredByRefresh...)

	return config, err
}

func getStringValue(cm *corev1.ConfigMap, key string, defaultValue string) string {
	if cm == nil {
		return defaultValue
	}

	if value, ok := cm.Data[key]; ok {
		return value
	}

	return defaultValue
}

func LoadSecret(c client.Client, namespace string, name string) (DBParams, error) {
	secret, err := utils.FetchSecret(c, namespace, name)

	if err != nil {
		return DBParams{}, err
	}

	params, err := ParseDBSecret(secret)
	return params, err
}

func getIntValue(cm *corev1.ConfigMap, key string, defaultValue int64) (int64, error) {
	if cm == nil {
		return defaultValue, nil
	}

	if value, ok := cm.Data[key]; ok {
		if parsed, err := strconv.ParseInt(value, 10, 64); err != nil {
			return -1, fmt.Errorf(`"%s" is not a valid value for "%s"`, value, key)
		} else {
			return parsed, nil
		}
	}

	return defaultValue, nil
}

func getValidationConfig(instance *xjoin.XJoinPipeline,
	cm *corev1.ConfigMap,
	prefix string,
	defaultValue ValidationConfiguration) (ValidationConfiguration, error) {
	var (
		err    error
		result = ValidationConfiguration{}
	)

	if result.Interval, err = getIntValue(cm, fmt.Sprintf("%s%s", prefix, validationInterval), defaultValue.Interval); err != nil {
		return result, err
	}

	if result.AttemptsThreshold, err = getIntValue(cm, fmt.Sprintf("%s%s", prefix, validationAttemptsThreshold), defaultValue.AttemptsThreshold); err != nil {
		return result, err
	}

	if instance != nil && instance.Spec.ValidationThreshold != nil {
		result.PercentageThreshold = *instance.Spec.ValidationThreshold
	} else if result.PercentageThreshold, err = getIntValue(cm, fmt.Sprintf("%s%s", prefix, validationPercentageThreshold), defaultValue.PercentageThreshold); err != nil {
		return result, err
	}

	return result, err
}
