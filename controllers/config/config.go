package config

import (
	"context"
	"errors"
	"fmt"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"time"
)

var log = logger.NewLogger("config")

// These keys are excluded when computing a configMap hash.
// Therefore, if they change that won't trigger a pipeline refresh
var keysIgnoredByRefresh []string

type Config struct {
	hbiDBSecret         *corev1.Secret
	elasticSearchSecret *corev1.Secret
	instance            *xjoin.XJoinPipeline
	configMap           *corev1.ConfigMap
	Parameters          Parameters
	ParametersMap       map[string]interface{}
	client              client.Client
}

func NewConfig(instance *xjoin.XJoinPipeline, client client.Client) (*Config, error) {
	config := Config{}

	config.Parameters = NewXJoinConfiguration()
	cm, err := utils.FetchConfigMap(client, instance.Namespace, "xjoin")
	if err != nil {
		return &config, err
	}
	config.configMap = cm
	config.instance = instance
	config.client = client

	hbiDBSecretNameVal, err := config.parameterValue(config.Parameters.HBIDBSecretName)
	if err != nil {
		return &config, err
	}
	hbiDBSecretName := hbiDBSecretNameVal.Interface().(Parameter)
	instance.Status.HBIDBSecretName = hbiDBSecretName.String()
	config.hbiDBSecret, err = utils.FetchSecret(client, instance.Namespace, hbiDBSecretName.String())
	if err != nil {
		return &config, err
	}

	elasticSearchSecretVal, err := config.parameterValue(config.Parameters.ElasticSearchSecretName)
	if err != nil {
		return &config, err
	}
	elasticSearchSecretName := elasticSearchSecretVal.Interface().(Parameter)
	instance.Status.ElasticSearchSecretName = elasticSearchSecretName.String()
	config.elasticSearchSecret, err = utils.FetchSecret(client, instance.Namespace, elasticSearchSecretName.String())
	if err != nil {
		return &config, err
	}

	err = config.buildXJoinConfig()
	if err != nil {
		return &config, err
	}

	return &config, nil
}

func (config *Config) parameterValue(param Parameter) (reflect.Value, error) {
	emptyValue := reflect.Value{}

	if param.SpecKey != "" {
		specReflection := reflect.ValueOf(&config.instance.Spec).Elem()
		paramValue := specReflection.FieldByName(param.SpecKey).Interface()
		err := param.SetValue(paramValue)
		if err != nil {
			return emptyValue, err
		}
	}

	if param.Secret == secretTypes.elasticSearch && config.elasticSearchSecret != nil && param.value == nil {
		value, err := config.readSecretValue(config.elasticSearchSecret, param.SecretKey)
		if err != nil {
			return emptyValue, err
		}

		err = param.SetValue(value)
		if err != nil {
			return emptyValue, err
		}
	}

	if param.Secret == secretTypes.hbiDB && config.hbiDBSecret != nil && param.value == nil {
		value, err := config.readSecretValue(config.hbiDBSecret, param.SecretKey)
		if err != nil {
			return emptyValue, err
		}
		err = param.SetValue(value)
		if err != nil {
			return emptyValue, err
		}
	}

	if param.value == nil && param.ConfigMapKey != "" {
		var value interface{}
		var err error

		if param.Type == reflect.String {
			value = config.getStringValue(param.ConfigMapKey, param.DefaultValue.(string))
		} else if param.Type == reflect.Int {
			value, err = config.getIntValue(param.ConfigMapKey, param.DefaultValue.(int))
		} else if param.Type == reflect.Bool {
			value, err = config.getBoolValue(param.ConfigMapKey, param.DefaultValue.(bool))
		}

		if err != nil {
			return emptyValue, err
		}

		err = param.SetValue(value)
		if err != nil {
			return emptyValue, err
		}
	}

	return reflect.ValueOf(param), nil
}

//Unable to pass ephemeral environment's kafka/connect cluster name into the deployment template
func (config *Config) buildEphemeralConfig() (err error) {
	log.Info("Loading Kafka parameters for ephemeral environment")

	var connectGVK = schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Kind:    "KafkaConnectList",
		Version: "v1beta2",
	}

	connect := &unstructured.UnstructuredList{}
	connect.SetGroupVersionKind(connectGVK)

	err = config.Parameters.ConnectClusterNamespace.SetValue(config.instance.Namespace)
	if err != nil {
		return
	}

	err = config.Parameters.KafkaClusterNamespace.SetValue(config.instance.Namespace)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err = config.client.List(
		ctx,
		connect,
		client.InNamespace(config.instance.Namespace))
	if err != nil {
		return
	}

	if len(connect.Items) != 1 {
		return errors.New("invalid number of connect instances found: " + strconv.Itoa(len(connect.Items)))
	}

	err = config.Parameters.ConnectCluster.SetValue(connect.Items[0].GetName())
	if err != nil {
		return
	}

	var kafkaGVK = schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Kind:    "KafkaList",
		Version: "v1beta2",
	}

	kafka := &unstructured.UnstructuredList{}
	kafka.SetGroupVersionKind(kafkaGVK)

	err = config.client.List(
		ctx,
		kafka,
		client.InNamespace(config.instance.Namespace))
	if err != nil {
		return
	}

	if len(kafka.Items) != 1 {
		return errors.New("invalid number of kafka instances found: " + strconv.Itoa(len(kafka.Items)))
	}

	err = config.Parameters.KafkaCluster.SetValue(kafka.Items[0].GetName())
	if err != nil {
		return
	}

	err = config.Parameters.ElasticSearchURL.SetValue(
		"http://xjoin-elasticsearch-es-default." + config.instance.Namespace + ".svc:9200")
	if err != nil {
		return
	}

	err = config.Parameters.ElasticSearchUsername.SetValue("elastic")
	if err != nil {
		return
	}

	esSecret, err := utils.FetchSecret(config.client, config.instance.Namespace, "xjoin-elasticsearch-es-elastic-user")
	if err != nil {
		return err
	}

	password, err := config.readSecretValue(esSecret, []string{"elastic"})
	err = config.Parameters.ElasticSearchPassword.SetValue(password)
	if err != nil {
		return
	}

	config.ParametersMap["KafkaClusterNamespace"] = config.Parameters.KafkaClusterNamespace.String()
	config.ParametersMap["KafkaCluster"] = config.Parameters.KafkaCluster.String()
	config.ParametersMap["ConnectClusterNamespace"] = config.Parameters.ConnectClusterNamespace.String()
	config.ParametersMap["ConnectCluster"] = config.Parameters.ConnectCluster.String()
	config.ParametersMap["ElasticSearchURL"] = config.Parameters.ElasticSearchURL.String()
	config.ParametersMap["ElasticSearchUsername"] = config.Parameters.ElasticSearchUsername.String()
	config.ParametersMap["ElasticSearchPassword"] = config.Parameters.ElasticSearchPassword.String()

	return
}

func (config *Config) buildXJoinConfig() error {

	configReflection := reflect.ValueOf(&config.Parameters).Elem()
	parametersMap := make(map[string]interface{})

	for i := 0; i < configReflection.NumField(); i++ {
		param := configReflection.Field(i).Interface().(Parameter)
		if param.DefaultValue == nil {
			continue
		}
		value, err := config.parameterValue(param)
		if err != nil {
			return err
		}
		configReflection.Field(i).Set(value)

		updatedParameter := configReflection.Field(i).Interface().(Parameter)
		parametersMap[configReflection.Type().Field(i).Name] = updatedParameter.Value()
	}
	config.ParametersMap = parametersMap

	if config.Parameters.Ephemeral.Bool() == true {
		err := config.buildEphemeralConfig()
		if err != nil {
			return err
		}
	}

	configMapHash, err := utils.ConfigMapHash(config.configMap, keysIgnoredByRefresh...)
	if err != nil {
		return err
	}
	err = config.Parameters.ConfigMapVersion.SetValue(configMapHash)
	if err != nil {
		return err
	}

	dbSecretHash, err := utils.SecretHash(config.hbiDBSecret)
	if err != nil {
		return err
	}
	err = config.Parameters.HBIDBSecretVersion.SetValue(dbSecretHash)
	if err != nil {
		return err
	}

	esSecretHash, err := utils.SecretHash(config.elasticSearchSecret)
	if err != nil {
		return err
	}
	err = config.Parameters.ElasticSearchSecretVersion.SetValue(esSecretHash)
	if err != nil {
		return err
	}

	return nil
}

func (config *Config) getBoolValue(key string, defaultValue bool) (bool, error) {
	if config.configMap == nil {
		return defaultValue, nil
	}

	if value, ok := config.configMap.Data[key]; ok {
		if parsed, err := strconv.ParseBool(value); err != nil {
			return false, fmt.Errorf(`"%s" is not a valid value for "%s"`, value, key)
		} else {
			return parsed, nil
		}
	}

	return defaultValue, nil
}

func (config *Config) getStringValue(key string, defaultValue string) string {
	if config.configMap == nil {
		return defaultValue
	}

	if value, ok := config.configMap.Data[key]; ok {
		return value
	}

	return defaultValue
}

func (config *Config) getIntValue(key string, defaultValue int) (int, error) {
	if config.configMap == nil {
		return defaultValue, nil
	}

	value, ok := config.configMap.Data[key]

	if ok {
		if parsed, err := strconv.ParseInt(value, 10, 64); err != nil {
			return -1, fmt.Errorf(`"%s" is not a valid value for "%s"`, value, key)
		} else {
			return int(parsed), nil
		}
	} else {
		log.Debug("Key missing from configmap, falling back to default value", "key", key)
	}

	return defaultValue, nil
}

func (config *Config) readSecretValue(secret *corev1.Secret, keys []string) (value string, err error) {
	for _, key := range keys {
		value = string(secret.Data[key])
		if value != "" {
			break
		}
	}
	return
}
