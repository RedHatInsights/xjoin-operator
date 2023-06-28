package parameters

import (
	"github.com/go-errors/errors"
	xjoinUtils "github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	. "github.com/redhatinsights/xjoin-operator/controllers/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

type CommonParameters struct {
	Pause                        Parameter
	Version                      Parameter
	ConnectCluster               Parameter
	ConnectClusterNamespace      Parameter
	KafkaTopicPartitions         Parameter
	KafkaTopicReplicas           Parameter
	KafkaTopicCleanupPolicy      Parameter
	KafkaTopicMinCompactionLagMS Parameter
	KafkaTopicRetentionBytes     Parameter
	KafkaTopicRetentionMS        Parameter
	KafkaTopicMessageBytes       Parameter
	KafkaTopicCreationTimeout    Parameter
	KafkaCluster                 Parameter
	KafkaClusterNamespace        Parameter
	SchemaRegistryProtocol       Parameter
	SchemaRegistryHost           Parameter
	SchemaRegistryPort           Parameter
	AvroSchema                   Parameter
}

func BuildCommonParameters() CommonParameters {
	p := CommonParameters{
		Pause: Parameter{
			SpecKey:      "Pause",
			DefaultValue: false,
			Type:         reflect.Bool,
		},
		Version: Parameter{
			Type:         reflect.String,
			SpecKey:      "Version",
			DefaultValue: "1",
		},

		//connect cluster
		ConnectCluster: Parameter{
			SpecKey:       "ConnectCluster",
			ConfigMapKey:  "connect.cluster",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "connect",
			Type:          reflect.String,
			Ephemeral: func(manager Manager) (interface{}, error) {
				var connectGVK = schema.GroupVersionKind{
					Group:   "kafka.strimzi.io",
					Kind:    "KafkaConnectList",
					Version: "v1beta2",
				}

				connect := &unstructured.UnstructuredList{}
				connect.SetGroupVersionKind(connectGVK)

				ctx, cancel := xjoinUtils.DefaultContext()
				defer cancel()
				err := manager.Client.List(
					ctx,
					connect,
					client.InNamespace(manager.Namespace))

				if err != nil {
					return nil, err
				}

				connectClusterName := ""
				if len(connect.Items) == 0 {
					return nil, errors.New("invalid number of connect instances found: " + strconv.Itoa(len(connect.Items)))
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
						return nil, errors.New("no kafka connect instance found. Is there a valid ClowdEnv setup?")
					}
				} else {
					connectClusterName = connect.Items[0].GetName()
				}

				return connectClusterName, nil
			},
		},
		ConnectClusterNamespace: Parameter{
			SpecKey:       "ConnectClusterNamespace",
			ConfigMapKey:  "connect.cluster.namespace",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "test",
			Type:          reflect.String,
			Ephemeral: func(manager Manager) (interface{}, error) {
				return manager.Namespace, nil
			},
		},

		//kafka cluster
		KafkaCluster: Parameter{
			SpecKey:       "KafkaCluster",
			ConfigMapKey:  "kafka.cluster",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "kafka",
			Type:          reflect.String,
			Ephemeral: func(manager Manager) (interface{}, error) {
				var kafkaGVK = schema.GroupVersionKind{
					Group:   "kafka.strimzi.io",
					Kind:    "KafkaList",
					Version: "v1beta2",
				}

				kafka := &unstructured.UnstructuredList{}
				kafka.SetGroupVersionKind(kafkaGVK)

				ctx, cancel := xjoinUtils.DefaultContext()
				defer cancel()
				err := manager.Client.List(
					ctx,
					kafka,
					client.InNamespace(manager.Namespace))

				if err != nil {
					return nil, err
				}

				if len(kafka.Items) != 1 {
					return nil, errors.New("invalid number of kafka instances found: " + strconv.Itoa(len(kafka.Items)))
				}

				return kafka.Items[0].GetName(), nil
			},
		},
		KafkaClusterNamespace: Parameter{
			SpecKey:       "KafkaClusterNamespace",
			ConfigMapKey:  "kafka.cluster.namespace",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "test",
			Type:          reflect.String,
			Ephemeral: func(manager Manager) (interface{}, error) {
				return manager.Namespace, nil
			},
		},

		//kafka topic
		KafkaTopicPartitions: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "kafka.topic.partitions",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},
		KafkaTopicReplicas: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "kafka.topic.replicas",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  1,
		},
		KafkaTopicCleanupPolicy: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.topic.cleanup.policy",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "compact,delete",
		},
		KafkaTopicMinCompactionLagMS: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.topic.min.compaction.lag.ms",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "3600000",
		},
		KafkaTopicRetentionBytes: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.topic.retention.bytes",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "5368709120",
		},
		KafkaTopicRetentionMS: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.topic.retention.ms",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "2678400001",
		},
		KafkaTopicMessageBytes: Parameter{
			Type:          reflect.String,
			ConfigMapKey:  "kafka.topic.max.message.bytes",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  "2097176",
		},
		KafkaTopicCreationTimeout: Parameter{
			Type:          reflect.Int,
			ConfigMapKey:  "kafka.topic.creation.timeout",
			ConfigMapName: "xjoin-generic",
			DefaultValue:  300,
		},

		//avro schema
		SchemaRegistryProtocol: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "schemaregistry.protocol",
			DefaultValue:  "http",
		},
		SchemaRegistryHost: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "schemaregistry.host",
			DefaultValue:  "apicurio.test.svc",
			Ephemeral: func(manager Manager) (interface{}, error) {
				return "xjoin-apicurio-service." + manager.Namespace + ".svc", nil
			},
		},
		SchemaRegistryPort: Parameter{
			Type:          reflect.String,
			ConfigMapName: "xjoin-generic",
			ConfigMapKey:  "schemaregistry.port",
			DefaultValue:  "10001",
			Ephemeral: func(manager Manager) (interface{}, error) {
				return "10000", nil
			},
		},
		AvroSchema: Parameter{
			Type:         reflect.String,
			SpecKey:      "AvroSchema",
			DefaultValue: "{}",
		},
	}

	return p
}
