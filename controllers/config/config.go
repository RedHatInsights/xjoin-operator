package config

import (
	"fmt"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

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

	if value, ok := config.configMap.Data[key]; ok {
		if parsed, err := strconv.ParseInt(value, 10, 64); err != nil {
			return -1, fmt.Errorf(`"%s" is not a valid value for "%s"`, value, key)
		} else {
			return int(parsed), nil
		}
	}

	return defaultValue, nil
}

func (config *Config) readSecretValue(secret *corev1.Secret, key string) (string, error) {
	value := secret.Data[key]
	if value == nil || string(value) == "" {
		return "", fmt.Errorf("%s missing from %s secret", key, secret.ObjectMeta.Name)
	}

	return string(value), nil
}
