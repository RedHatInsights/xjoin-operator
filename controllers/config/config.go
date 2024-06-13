package config

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-errors/errors"
	xjoinUtils "github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These keys are excluded when computing a configMap hash.
// Therefore, if they change that won't trigger a pipeline refresh
var keysIgnoredByRefresh []string

type Config struct {
	hbiDBSecret          *corev1.Secret
	elasticSearchSecret  *corev1.Secret
	schemaRegistrySecret *corev1.Secret
	instance             *xjoin.XJoinPipeline
	configMap            *corev1.ConfigMap
	Parameters           Parameters
	ParametersMap        map[string]interface{}
	client               client.Client
	log                  logger.Log
}

func NewConfig(instance *xjoin.XJoinPipeline, client client.Client, ctx context.Context, log logger.Log) (*Config, error) {
	config := Config{}

	config.log = log

	config.Parameters = NewXJoinConfiguration()
	cm, err := k8sUtils.FetchConfigMap(client, instance.Namespace, "xjoin", ctx)
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
	config.hbiDBSecret, err = k8sUtils.FetchSecret(client, instance.Namespace, hbiDBSecretName.String(), ctx)
	if err != nil {
		return &config, err
	}

	elasticSearchSecretVal, err := config.parameterValue(config.Parameters.ElasticSearchSecretName)
	if err != nil {
		return &config, err
	}
	elasticSearchSecretName := elasticSearchSecretVal.Interface().(Parameter)
	instance.Status.ElasticSearchSecretName = elasticSearchSecretName.String()
	config.elasticSearchSecret, err = k8sUtils.FetchSecret(client, instance.Namespace, elasticSearchSecretName.String(), ctx)
	if err != nil {
		return &config, err
	}

	schemaRegistrySecretNameVal, err := config.parameterValue(config.Parameters.SchemaRegistrySecretName)
	if err != nil {
		return &config, err
	}
	schemaRegistrySecretName := schemaRegistrySecretNameVal.Interface().(Parameter)
	config.schemaRegistrySecret, err = k8sUtils.FetchSecret(client, instance.Namespace, schemaRegistrySecretName.String(), ctx)
	if err != nil {
		return &config, err
	}

	err = config.buildXJoinConfig(ctx)
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

	if param.Secret == secretTypes.schemaRegistry && config.schemaRegistrySecret != nil && param.value == nil {
		value, err := config.readSecretValue(config.schemaRegistrySecret, param.SecretKey)
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

func (config *Config) checkIfManagedKafka(ctx context.Context) (isManaged bool, err error) {
	var clowdenvGVK = schema.GroupVersionKind{
		Group:   "cloud.redhat.com",
		Kind:    "ClowdEnvironment",
		Version: "v1alpha1",
	}
	clowdenv := unstructured.Unstructured{}
	clowdenv.SetGroupVersionKind(clowdenvGVK)

	err = config.client.Get(ctx, types.NamespacedName{Name: "env-" + config.instance.Namespace}, &clowdenv)
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	clowdenvSpec := clowdenv.Object["spec"].(map[string]interface{})
	clowdenvProviders := clowdenvSpec["providers"].(map[string]interface{})
	clowdenvKafka := clowdenvProviders["kafka"].(map[string]interface{})
	clowdenvKafkaMode := clowdenvKafka["mode"].(string)

	config.log.Info("ClowdEnvironment Kafka Mode: " + clowdenvKafkaMode)

	if clowdenvKafkaMode == "ephem-msk" {
		isManaged = true
	} else {
		isManaged = false
	}

	return isManaged, err
}

// Unable to pass ephemeral environment's kafka/connect cluster name into the deployment template
func (config *Config) buildEphemeralConfig(ctx context.Context) (err error) {
	config.log.Info("Loading Kafka parameters for ephemeral environment: " + config.instance.Namespace)

	isManagedKafka, err := config.checkIfManagedKafka(ctx)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	if isManagedKafka {
		config.log.Info("Using Managed Kafka instance")
		//set config.parameters value to true
		err = config.Parameters.ManagedKafka.SetValue(true)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		config.ParametersMap["ManagedKafka"] = true
	}

	if config.Parameters.ManagedKafka.Bool() {
		resourceNamePrefix := strings.ReplaceAll(config.instance.Namespace, "-", ".")
		err = config.Parameters.ResourceNamePrefix.SetValue(resourceNamePrefix)
		if err != nil {
			return errors.Wrap(err, 0)
		}
		config.ParametersMap["ResourceNamePrefix"] = resourceNamePrefix
	}

	var connectGVK = schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Kind:    "KafkaConnectList",
		Version: "v1beta2",
	}

	connect := &unstructured.UnstructuredList{}
	connect.SetGroupVersionKind(connectGVK)

	if config.instance.Spec.ConnectClusterNamespace == nil {
		err = config.Parameters.ConnectClusterNamespace.SetValue(config.instance.Namespace)
		if err != nil {
			return err
		}
	}

	if config.instance.Spec.KafkaClusterNamespace == nil {
		err = config.Parameters.KafkaClusterNamespace.SetValue(config.instance.Namespace)
		if err != nil {
			return err
		}
	}

	if config.instance.Spec.ElasticSearchNamespace == nil {
		err = config.Parameters.ElasticSearchNamespace.SetValue(config.instance.Namespace)
		if err != nil {
			return err
		}
	}

	ctx, cancel := xjoinUtils.DefaultContext()
	defer cancel()
	err = config.client.List(
		ctx,
		connect,
		client.InNamespace(config.Parameters.ConnectClusterNamespace.String()))
	if err != nil {
		return err
	}

	connectClusterName := ""
	if len(connect.Items) == 0 {
		return errors.New("invalid number of connect instances found: " + strconv.Itoa(len(connect.Items)))
	} else if len(connect.Items) > 1 {
		for _, connectInstance := range connect.Items {
			ownerRefs := connectInstance.GetOwnerReferences()

			for _, ref := range ownerRefs {
				if ref.Kind == "ClowdEnvironment" {
					connectClusterName = connectInstance.GetName()
				}
			}
		}

		if connectClusterName == "" {
			return errors.New("no kafka connect instance found. Is there a valid ClowdEnv setup?")
		}
	} else {
		connectClusterName = connect.Items[0].GetName()
	}

	err = config.Parameters.ConnectCluster.SetValue(connectClusterName)
	if err != nil {
		return err
	}

	if !isManagedKafka {
		config.log.Info("Using Strimzi Kafka instance")
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
			client.InNamespace(config.Parameters.KafkaClusterNamespace.String()))
		if err != nil {
			return err
		}

		if len(kafka.Items) != 1 {
			return errors.New("invalid number of kafka instances found: " + strconv.Itoa(len(kafka.Items)))
		}

		err = config.Parameters.KafkaCluster.SetValue(kafka.Items[0].GetName())
		if err != nil {
			return err
		}
	}

	err = config.Parameters.ElasticSearchURL.SetValue(
		"http://xjoin-elasticsearch-es-default." + config.Parameters.ElasticSearchNamespace.String() + ".svc:9200")
	if err != nil {
		return err
	}

	err = config.Parameters.ElasticSearchUsername.SetValue("elastic")
	if err != nil {
		return err
	}

	esSecret, err := k8sUtils.FetchSecret(
		config.client, config.Parameters.ElasticSearchNamespace.String(), "xjoin-elasticsearch-es-elastic-user", ctx)
	if err != nil {
		return err
	}

	password, err := config.readSecretValue(esSecret, []string{"elastic"})
	if err != nil {
		return err
	}
	err = config.Parameters.ElasticSearchPassword.SetValue(password)
	if err != nil {
		return err
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

func (config *Config) buildXJoinConfig(ctx context.Context) error {

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

	if config.Parameters.Ephemeral.Bool() {
		err := config.buildEphemeralConfig(ctx)
		if err != nil {
			return err
		}
	}

	configMapHash, err := k8sUtils.ConfigMapHash(config.configMap, keysIgnoredByRefresh...)
	if err != nil {
		return err
	}
	err = config.Parameters.ConfigMapVersion.SetValue(configMapHash)
	if err != nil {
		return err
	}

	dbSecretHash, err := k8sUtils.SecretHash(config.hbiDBSecret)
	if err != nil {
		return err
	}
	err = config.Parameters.HBIDBSecretVersion.SetValue(dbSecretHash)
	if err != nil {
		return err
	}

	esSecretHash, err := k8sUtils.SecretHash(config.elasticSearchSecret)
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
		config.log.Debug("Key missing from configmap, falling back to default value", "key", key)
	}

	return defaultValue, nil
}

func (config *Config) readSecretValue(secret *corev1.Secret, keys []string) (value string, err error) {
	return k8sUtils.ReadSecretValue(secret, keys)
}
