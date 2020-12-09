package config

import (
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These keys are excluded when computing a ConfigMap hash.
// Therefore, if they change that won't trigger a pipeline refresh
var keysIgnoredByRefresh []string

func BuildXJoinConfig(instance *xjoin.XJoinPipeline, cm *corev1.ConfigMap) (*XJoinConfiguration, error) {
	var err error
	config := &XJoinConfiguration{}

	if instance != nil && instance.Spec.ConnectCluster != nil {
		config.ConnectCluster = *instance.Spec.ConnectCluster
	} else {
		config.ConnectCluster = getStringValue(cm, "connect.cluster", defaultConnectCluster)
	}

	//TODO: handle a secret for ES params
	config.ElasticSearchURL = getStringValue(cm, "elasticsearch.url", defaultElasticSearchURL)
	config.ElasticSearchUsername = getStringValue(cm, "elasticsearch.url", defaultElasticSearchUsername)
	config.ElasticSearchPassword = getStringValue(cm, "elasticsearch.url", defaultElasticSearchPassword)

	config.DebeziumConnectorTemplate = getStringValue(
		cm, "debezium.connector.config", defaultDebeziumConnectorTemplate)
	config.ElasticSearchConnectorTemplate = getStringValue(
		cm, "elasticsearch.connector.config", defaultElasticSearchConnectorTemplate)

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
