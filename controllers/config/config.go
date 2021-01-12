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

//TODO make these params
const (
	elasticSearchSecret = "xjoin-elasticsearch"
	hbiDBSecret         = "host-inventory-db"
)

// These keys are excluded when computing a ConfigMap hash.
// Therefore, if they change that won't trigger a pipeline refresh
var keysIgnoredByRefresh []string

func readSecretValue(secret *corev1.Secret, key string) (string, error) {
	value := secret.Data[key]
	if value == nil || string(value) == "" {
		return "", fmt.Errorf("%s missing from %s secret", key, secret.ObjectMeta.Name)
	}

	return string(value), nil
}

func BuildXJoinConfig(instance *xjoin.XJoinPipeline, client client.Client) (XJoinConfiguration, error) {
	config := NewXJoinConfiguration()
	cm, err := utils.FetchConfigMap(client, instance.Namespace, "xjoin")

	hbiSecret, err := utils.FetchSecret(client, instance.Namespace, hbiDBSecret)
	esSecret, err := utils.FetchSecret(client, instance.Namespace, elasticSearchSecret)

	if err != nil {
		return config, err
	}

	configReflection := reflect.ValueOf(&config).Elem()

	for i := 0; i < configReflection.NumField(); i++ {
		param := configReflection.Field(i).Interface().(Parameter)
		if param.DefaultValue == nil {
			continue
		}

		if param.SpecKey != "" {
			specReflection := reflect.ValueOf(&instance.Spec).Elem()
			paramValue := specReflection.FieldByName(param.SpecKey).Interface()
			err = param.SetValue(paramValue)
			if err != nil {
				return config, err
			}
		}

		if param.Secret == elasticSearchSecret {
			value, err := readSecretValue(esSecret, param.SecretKey)
			if err != nil {
				return config, err
			}

			err = param.SetValue(value)
			if err != nil {
				return config, err
			}
		}

		if param.Secret == hbiDBSecret {
			value, err := readSecretValue(hbiSecret, param.SecretKey)
			if err != nil {
				return config, err
			}
			err = param.SetValue(value)
			if err != nil {
				return config, err
			}
		}

		if param.value == nil && param.ConfigMapKey != "" {
			var value interface{}
			if param.Type == reflect.String {
				value = getStringValue(cm, param.ConfigMapKey, param.DefaultValue.(string))
			} else if param.Type == reflect.Int {
				value, err = getIntValue(cm, param.ConfigMapKey, param.DefaultValue.(int))
			} else if param.Type == reflect.Bool {
				value, err = getBoolValue(cm, param.ConfigMapKey, param.DefaultValue.(bool))
			}

			if err != nil {
				return config, err
			}

			err = param.SetValue(value)
			if err != nil {
				return config, err
			}
		}

		configReflection.Field(i).Set(reflect.ValueOf(param))
	}

	err = config.ConfigMapVersion.SetValue(utils.ConfigMapHash(cm, keysIgnoredByRefresh...))
	if err != nil {
		return config, err
	}

	return config, nil
}

func getBoolValue(cm *corev1.ConfigMap, key string, defaultValue bool) (bool, error) {
	if cm == nil {
		return defaultValue, nil
	}

	if value, ok := cm.Data[key]; ok {
		if parsed, err := strconv.ParseBool(value); err != nil {
			return false, fmt.Errorf(`"%s" is not a valid value for "%s"`, value, key)
		} else {
			return parsed, nil
		}
	}

	return defaultValue, nil
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

func getIntValue(cm *corev1.ConfigMap, key string, defaultValue int) (int, error) {
	if cm == nil {
		return defaultValue, nil
	}

	if value, ok := cm.Data[key]; ok {
		if parsed, err := strconv.ParseInt(value, 10, 64); err != nil {
			return -1, fmt.Errorf(`"%s" is not a valid value for "%s"`, value, key)
		} else {
			return int(parsed), nil
		}
	}

	return defaultValue, nil
}
