package config

import (
	"context"
	"errors"
	"fmt"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	v1 "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

type Spec interface{}

type Manager struct {
	Parameters     interface{}
	client         client.Client
	ctx            context.Context
	configMapNames []string
	secretNames    []string
	namespace      string
	configMaps     map[string]v1.ConfigMap
	secrets        map[string]v1.Secret
	spec           interface{}
}

type ManagerOptions struct {
	Client         client.Client
	Parameters     interface{}
	ConfigMapNames []string
	SecretNames    []string
	Namespace      string
	Spec           Spec
	Context        context.Context
}

func NewManager(opts ManagerOptions) (*Manager, error) {
	configMaps := make(map[string]v1.ConfigMap)
	managerSecrets := make(map[string]v1.Secret)

	if opts.Context == nil {
		return nil, errors.New("context is required")
	}

	return &Manager{
		client:         opts.Client,
		Parameters:     opts.Parameters,
		configMapNames: opts.ConfigMapNames,
		secretNames:    opts.SecretNames,
		namespace:      opts.Namespace,
		spec:           opts.Spec,
		configMaps:     configMaps,
		secrets:        managerSecrets,
		ctx:            opts.Context,
	}, nil
}

func (m *Manager) Parse() (err error) {
	err = m.loadConfigMaps()
	if err != nil {
		return
	}

	err = m.loadSecrets()
	if err != nil {
		return
	}

	parameters := reflect.ValueOf(m.Parameters).Elem()
	for i := 0; i < parameters.NumField(); i++ {
		param := parameters.Field(i).Interface().(Parameter)
		value, err := m.parseParameterValue(param)
		if err != nil {
			return err
		}
		err = param.SetValue(value)
		if err != nil {
			return err
		}
		parameters.Field(i).Set(reflect.ValueOf(param))
	}

	return
}

func (m *Manager) loadConfigMaps() (err error) {
	for _, name := range m.configMapNames {
		cm, err := utils.FetchConfigMap(m.client, m.namespace, name, m.ctx)
		if err != nil {
			return err
		}
		if cm == nil {
			return errors.New(fmt.Sprintf("configmap not found: %s", name))
		}
		m.configMaps[name] = *cm
	}
	return
}

func (m *Manager) loadSecrets() (err error) {
	for _, name := range m.secretNames {
		secret, err := utils.FetchSecret(m.client, m.namespace, name, m.ctx)
		if err != nil {
			return err
		}
		if secret == nil {
			return errors.New(fmt.Sprintf("secret not found: %s", name))
		}
		m.secrets[name] = *secret
	}
	return
}

// priority: spec > secret > configmap > default
func (m *Manager) parseParameterValue(param Parameter) (value interface{}, err error) {
	if param.SpecKey != "" {
		specReflection := reflect.ValueOf(&m.spec).Elem().Elem()
		field := specReflection.FieldByName(param.SpecKey)

		if !field.IsValid() {
			log.Debug(fmt.Sprintf("key %s not found in spec", param.SpecKey))
		} else {
			value = field.Interface()
			if err != nil {
				return
			}
		}
	}

	if param.Secret != "" && param.value == nil {
		if _, hasKey := m.secrets[param.Secret]; !hasKey {
			return nil, errors.New(fmt.Sprintf(
				"secret %s was not found. Did you register it when initializing the config.Manager?", param.Secret))
		}

		value, err = readSecretValue(m.secrets[param.Secret], param.SecretKey)

		if err != nil {
			return
		}
	}

	if param.ConfigMapKey != "" && param.value == nil {
		if _, hasKey := m.configMaps[param.ConfigMapName]; !hasKey {
			return nil, errors.New(fmt.Sprintf(
				"configmap %s was not found. Did you register it when initializing the config.Manager?", param.ConfigMapName))
		}

		if _, hasKey := m.configMaps[param.ConfigMapName].Data[param.ConfigMapKey]; !hasKey {
			value = param.DefaultValue
		} else {
			if param.Type == reflect.String {
				value = m.configMaps[param.ConfigMapName].Data[param.ConfigMapKey]
			} else if param.Type == reflect.Int {
				cmValue := m.configMaps[param.ConfigMapName].Data[param.ConfigMapKey]

				if parsed, err := strconv.ParseInt(cmValue, 10, 64); err != nil {
					return -1, fmt.Errorf(`"%s" is not a valid value for "%s" in configmap %s`,
						cmValue, param.ConfigMapKey, param.ConfigMapName)
				} else {
					value = int(parsed)
				}
			} else if param.Type == reflect.Bool {
				cmValue := m.configMaps[param.ConfigMapName].Data[param.ConfigMapKey]

				if parsed, err := strconv.ParseBool(cmValue); err != nil {
					return false, fmt.Errorf(`"%s" is not a valid value for "%s" in configmap %s`,
						cmValue, param.ConfigMapKey, param.ConfigMapName)
				} else {
					value = parsed
				}
			}
		}

		if err != nil {
			return
		}
	}

	if value == nil {
		return param.DefaultValue, nil
	}

	return
}

func readSecretValue(secret v1.Secret, keys []string) (value string, err error) {
	for _, key := range keys {
		value = string(secret.Data[key])
		if value != "" {
			break
		}
	}
	return
}
