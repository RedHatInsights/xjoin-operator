package config

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	v1 "k8s.io/api/core/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

type Spec interface{}

//SecretNames maps a user defined name (ManagerName) to the Kubernetes resource name (KubernetesName).
//            The user defined name is used when defining parameters.
type SecretNames struct {
	//Name of the Kubernetes secret resource
	KubernetesName string

	//User defined
	ManagerName string
}

type Manager struct {
	Parameters        interface{}
	Client            client.Client
	ctx               context.Context
	configMapNames    []string
	secretNames       []SecretNames
	ResourceNamespace string
	operatorNamespace string
	configMaps        map[string]v1.ConfigMap
	secrets           map[string]v1.Secret
	spec              interface{}
	log               logger.Log
	ephemeral         bool
}

type ManagerOptions struct {
	Client            client.Client
	Parameters        interface{}
	ConfigMapNames    []string
	SecretNames       []SecretNames
	ResourceNamespace string
	OperatorNamespace string
	Spec              Spec
	Context           context.Context
	Log               logger.Log
	Ephemeral         bool
}

func NewManager(opts ManagerOptions) (*Manager, error) {
	configMaps := make(map[string]v1.ConfigMap)
	managerSecrets := make(map[string]v1.Secret)

	if opts.Context == nil {
		return nil, errors.New("context is required")
	}

	return &Manager{
		Client:            opts.Client,
		Parameters:        opts.Parameters,
		configMapNames:    opts.ConfigMapNames,
		secretNames:       opts.SecretNames,
		ResourceNamespace: opts.ResourceNamespace,
		operatorNamespace: opts.OperatorNamespace,
		spec:              opts.Spec,
		configMaps:        configMaps,
		secrets:           managerSecrets,
		ctx:               opts.Context,
		log:               opts.Log,
		ephemeral:         opts.Ephemeral,
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
		if parameters.Field(i).Type() != reflect.TypeOf(Parameter{}) { //assume this is a composition struct
			commonParams := parameters.Field(i)
			for j := 0; j < commonParams.NumField(); j++ {
				commonParam := commonParams.Field(j).Interface().(Parameter)
				paramName := commonParams.Type().Field(j).Name
				value, err := m.parseParameterValue(paramName, commonParam)
				if err != nil {
					return errors.Wrap(err, 0)
				}
				err = commonParam.SetValue(value)
				if err != nil {
					return errors.Wrap(err, 0)
				}
				commonParams.Field(j).Set(reflect.ValueOf(commonParam))
			}
			parameters.Field(i).Set(commonParams)
		} else {
			param := parameters.Field(i).Interface().(Parameter)
			paramName := parameters.Type().Field(i).Name
			value, err := m.parseParameterValue(paramName, param)
			if err != nil {
				return errors.Wrap(err, 0)
			}
			err = param.SetValue(value)
			if err != nil {
				return errors.Wrap(err, 0)
			}
			parameters.Field(i).Set(reflect.ValueOf(param))

		}
	}

	return nil
}

func (m *Manager) loadConfigMaps() error {
	for _, name := range m.configMapNames {
		m.log.Debug(fmt.Sprintf("Loading config map '%s' from namespace '%s'", name, m.operatorNamespace))
		cm, err := k8sUtils.FetchConfigMap(m.Client, m.operatorNamespace, name, m.ctx)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		if cm == nil {
			return errors.Wrap(errors.New(fmt.Sprintf(
				"configmap '%s' not found in namespace '%s'", name, m.operatorNamespace)), 0)
		}
		m.configMaps[name] = *cm
	}
	return nil
}

func (m *Manager) loadSecrets() error {
	for _, name := range m.secretNames {
		m.log.Debug(fmt.Sprintf("Loading secret '%s' from namespace '%s'", name, m.ResourceNamespace))
		secret, err := k8sUtils.FetchSecret(m.Client, m.ResourceNamespace, name.KubernetesName, m.ctx)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		if secret == nil {
			return errors.Wrap(errors.New(fmt.Sprintf(
				"secret '%s' not found in namespace '%s'", name, m.ResourceNamespace)), 0)
		}
		m.secrets[name.ManagerName] = *secret
	}
	return nil
}

// priority: ephemeral > spec > secret > configmap > default
func (m *Manager) parseParameterValue(name string, param Parameter) (value interface{}, err error) {
	if param.Ephemeral != nil && m.ephemeral {
		m.log.Debug(fmt.Sprintf("Loading parameter %s from ephemeral", name))

		value, err = param.Ephemeral(*m)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
	}

	if param.SpecKey != "" && value == nil {
		m.log.Debug(fmt.Sprintf("Loading parameter %s from specKey %s", name, param.SpecKey))

		specReflection := reflect.ValueOf(&m.spec).Elem().Elem()
		field := specReflection.FieldByName(param.SpecKey)

		if !field.IsValid() {
			m.log.Debug(fmt.Sprintf("key %s not found in spec", param.SpecKey))
		} else if field.Type() == reflect.TypeOf(&v1alpha1.StringOrSecretParameter{}) {
			fieldParam := field.Interface().(*v1alpha1.StringOrSecretParameter)
			if fieldParam == nil {
				m.log.Warn(fmt.Sprintf("string or secret key %s not found in spec", param.SpecKey))
			} else if fieldParam.Value != "" {
				value = fieldParam.Value
			} else {
				secret := &v1.Secret{}
				err = m.Client.Get(m.ctx, client.ObjectKey{Name: fieldParam.ValueFrom.SecretKeyRef.Name, Namespace: m.ResourceNamespace}, secret)
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

	if param.Secret != "" && value == nil {
		m.log.Debug(fmt.Sprintf("Loading parameter %s from Secret: %s, SecretKey: param.SecretKey: %s",
			name, param.Secret, param.SecretKey))

		if _, hasKey := m.secrets[param.Secret]; !hasKey {
			return nil, errors.Wrap(errors.New(fmt.Sprintf(
				"secret %s was not found. Did you register it when initializing the config.Manager?", param.Secret)), 0)
		}

		value = readSecretValue(m.secrets[param.Secret], param.SecretKey)

		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
	}

	if param.ConfigMapKey != "" && value == nil {
		if _, hasKey := m.configMaps[param.ConfigMapName]; !hasKey {
			return nil, errors.Wrap(errors.New(fmt.Sprintf(
				"configmap %s was not found for key %s. Did you register it when initializing the config.Manager?",
				param.ConfigMapName, param.ConfigMapKey)), 0)
		}

		if _, hasKey := m.configMaps[param.ConfigMapName].Data[param.ConfigMapKey]; !hasKey {
			m.log.Debug(fmt.Sprintf("Using default value for parameter %s", name))

			value = param.DefaultValue
		} else {
			if param.Type == reflect.String {
				m.log.Debug(fmt.Sprintf("Loading parameter %s from ConfigMap: %s, ConfigMapKey %s",
					name, param.ConfigMapName, param.ConfigMapKey))

				value = m.configMaps[param.ConfigMapName].Data[param.ConfigMapKey]
			} else if param.Type == reflect.Int {
				m.log.Debug(fmt.Sprintf("Loading parameter %s from ConfigMap: %s, ConfigMapKey %s",
					name, param.ConfigMapName, param.ConfigMapKey))

				cmValue := m.configMaps[param.ConfigMapName].Data[param.ConfigMapKey]

				if parsed, err := strconv.ParseInt(cmValue, 10, 64); err != nil {
					return -1, errors.Wrap(fmt.Errorf(`"%s" is not a valid value for "%s" in configmap %s`,
						cmValue, param.ConfigMapKey, param.ConfigMapName), 0)
				} else {
					value = int(parsed)
				}
			} else if param.Type == reflect.Bool {
				m.log.Debug(fmt.Sprintf("Loading parameter %s from ConfigMap: %s, ConfigMapKey %s",
					name, param.ConfigMapName, param.ConfigMapKey))

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
		m.log.Debug(fmt.Sprintf("Using default value for parameter %s", name))
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

func ParametersToMap(p interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	value := reflect.ValueOf(p)
	typeOfValue := value.Type()

	for i := 0; i < value.NumField(); i++ {
		if value.Field(i).Type() != reflect.TypeOf(Parameter{}) { //assume this is a composition struct
			commonParams := value.Field(i)
			for j := 0; j < commonParams.NumField(); j++ {
				typeOfCommon := commonParams.Type()
				parameter := commonParams.Field(j).Interface().(Parameter)
				m[typeOfCommon.Field(j).Name] = parameter.Value()
			}
		} else {
			parameter := value.Field(i).Interface().(Parameter)
			m[typeOfValue.Field(i).Name] = parameter.Value()
		}
	}

	return m
}
