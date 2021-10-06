package config

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
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
	log            logger.Log
}

type ManagerOptions struct {
	Client         client.Client
	Parameters     interface{}
	ConfigMapNames []string
	SecretNames    []string
	Namespace      string
	Spec           Spec
	Context        context.Context
	Log            logger.Log
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
		log:            opts.Log,
	}, nil
}

func (m *Manager) Parse() error {
	err := m.loadConfigMaps()
	if err != nil {
		return errors.Wrap(err, 0)
	}

	err = m.loadSecrets()
	if err != nil {
		return errors.Wrap(err, 0)
	}

	parameters := reflect.ValueOf(m.Parameters).Elem()
	for i := 0; i < parameters.NumField(); i++ {
		param := parameters.Field(i).Interface().(Parameter)
		value, err := m.parseParameterValue(param)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		err = param.SetValue(value)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		parameters.Field(i).Set(reflect.ValueOf(param))
	}

	return nil
}

func (m *Manager) loadConfigMaps() error {
	for _, name := range m.configMapNames {
		cm, err := utils.FetchConfigMap(m.client, m.namespace, name, m.ctx)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		if cm == nil {
			return errors.Wrap(errors.New(fmt.Sprintf("configmap not found: %s", name)), 0)
		}
		m.configMaps[name] = *cm
	}
	return nil
}

func (m *Manager) loadSecrets() error {
	for _, name := range m.secretNames {
		secret, err := utils.FetchSecret(m.client, m.namespace, name, m.ctx)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		if secret == nil {
			return errors.Wrap(errors.New(fmt.Sprintf("secret not found: %s", name)), 0)
		}
		m.secrets[name] = *secret
	}
	return nil
}

// priority: spec > secret > configmap > default
func (m *Manager) parseParameterValue(param Parameter) (value interface{}, err error) {
	if param.SpecKey != "" {
		specReflection := reflect.ValueOf(&m.spec).Elem().Elem()
		field := specReflection.FieldByName(param.SpecKey)

		if !field.IsValid() {
			log.Debug(fmt.Sprintf("key %s not found in spec", param.SpecKey))
		} else if field.Type() == reflect.TypeOf(&v1alpha1.StringOrSecretParameter{}) {
			fieldParam := field.Interface().(*v1alpha1.StringOrSecretParameter)
			if fieldParam == nil {
				log.Warn(fmt.Sprintf("string or secret key %s not found in spec", param.SpecKey))
			} else if fieldParam.Value != "" {
				value = fieldParam.Value
			} else {
				secret := &v1.Secret{}
				err = m.client.Get(m.ctx, client.ObjectKey{Name: fieldParam.ValueFrom.SecretKeyRef.Name, Namespace: m.namespace}, secret)
				if err != nil {
					return value, errors.Wrap(err, 0)
				}
				value = readSecretValue(*secret, []string{fieldParam.ValueFrom.SecretKeyRef.Key})
			}
		} else {
			value = field.Interface()
			if err != nil {
				return value, errors.Wrap(err, 0)
			}
		}
	}

	if param.Secret != "" && param.value == nil {
		if _, hasKey := m.secrets[param.Secret]; !hasKey {
			return nil, errors.Wrap(errors.New(fmt.Sprintf(
				"secret %s was not found. Did you register it when initializing the config.Manager?", param.Secret)), 0)
		}

		value = readSecretValue(m.secrets[param.Secret], param.SecretKey)

		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
	}

	if param.ConfigMapKey != "" && param.value == nil {
		if _, hasKey := m.configMaps[param.ConfigMapName]; !hasKey {
			return nil, errors.Wrap(errors.New(fmt.Sprintf(
				"configmap %s was not found. Did you register it when initializing the config.Manager?", param.ConfigMapName)), 0)
		}

		if _, hasKey := m.configMaps[param.ConfigMapName].Data[param.ConfigMapKey]; !hasKey {
			value = param.DefaultValue
		} else {
			if param.Type == reflect.String {
				value = m.configMaps[param.ConfigMapName].Data[param.ConfigMapKey]
			} else if param.Type == reflect.Int {
				cmValue := m.configMaps[param.ConfigMapName].Data[param.ConfigMapKey]

				if parsed, err := strconv.ParseInt(cmValue, 10, 64); err != nil {
					return -1, errors.Wrap(fmt.Errorf(`"%s" is not a valid value for "%s" in configmap %s`,
						cmValue, param.ConfigMapKey, param.ConfigMapName), 0)
				} else {
					value = int(parsed)
				}
			} else if param.Type == reflect.Bool {
				cmValue := m.configMaps[param.ConfigMapName].Data[param.ConfigMapKey]

				if parsed, err := strconv.ParseBool(cmValue); err != nil {
					return false, errors.Wrap(fmt.Errorf(`"%s" is not a valid value for "%s" in configmap %s`,
						cmValue, param.ConfigMapKey, param.ConfigMapName), 0)
				} else {
					value = parsed
				}
			}
		}

		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
	}

	if value == nil {
		return param.DefaultValue, nil
	}

	return value, nil
}

func readSecretValue(secret v1.Secret, keys []string) (value string) {
	for _, key := range keys {
		value = string(secret.Data[key])
		if value != "" {
			break
		}
	}
	return
}
