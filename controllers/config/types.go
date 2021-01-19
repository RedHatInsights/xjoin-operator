package config

import "reflect"

type secrets struct {
	elasticSearch string
	hbiDB         string
}

var secretTypes = secrets{
	hbiDB:         "hbiDB",
	elasticSearch: "elasticsearch",
}

type Parameters struct {
	ResourceNamePrefix                Parameter
	ConnectCluster                    Parameter
	ConnectClusterNamespace           Parameter
	KafkaCluster                      Parameter
	ConfigMapVersion                  Parameter
	StandardInterval                  Parameter
	ValidationInterval                Parameter
	ValidationAttemptsThreshold       Parameter
	ValidationPercentageThreshold     Parameter
	ValidationInitInterval            Parameter
	ValidationInitAttemptsThreshold   Parameter
	ValidationInitPercentageThreshold Parameter
	ElasticSearchConnectorTemplate    Parameter
	ElasticSearchURL                  Parameter
	ElasticSearchUsername             Parameter
	ElasticSearchPassword             Parameter
	ElasticSearchTasksMax             Parameter
	ElasticSearchMaxInFlightRequests  Parameter
	ElasticSearchErrorsLogEnable      Parameter
	ElasticSearchMaxRetries           Parameter
	ElasticSearchRetryBackoffMS       Parameter
	ElasticSearchBatchSize            Parameter
	ElasticSearchMaxBufferedRecords   Parameter
	ElasticSearchLingerMS             Parameter
	DebeziumTemplate                  Parameter
	DebeziumTasksMax                  Parameter
	DebeziumMaxBatchSize              Parameter
	DebeziumQueueSize                 Parameter
	DebeziumPollIntervalMS            Parameter
	DebeziumErrorsLogEnable           Parameter
	HBIDBName                         Parameter
	HBIDBHost                         Parameter
	HBIDBPort                         Parameter
	HBIDBUser                         Parameter
	HBIDBPassword                     Parameter
	HBIDBSecretName                   Parameter
	ElasticSearchSecretName           Parameter
	KafkaTopicPartitions              Parameter
	KafkaTopicReplicas                Parameter
}

func NewXJoinConfiguration() Parameters {
	return Parameters{
		ConfigMapVersion: Parameter{
			DefaultValue: "",
			Type:         reflect.String,
		},
		ResourceNamePrefix: Parameter{
			SpecKey:      "ResourceNamePrefix",
			DefaultValue: "xjoin.inventory",
			Type:         reflect.String,
		},
		ConnectCluster: Parameter{
			SpecKey:      "ConnectCluster",
			ConfigMapKey: "connect.cluster",
			DefaultValue: "xjoin-kafka-connect-strimzi",
			Type:         reflect.String,
		},
		ConnectClusterNamespace: Parameter{
			SpecKey:      "ConnectClusterNamespace",
			ConfigMapKey: "connect.cluster.namespace",
			DefaultValue: "xjoin-operator-project",
			Type:         reflect.String,
		},
		HBIDBSecretName: Parameter{
			SpecKey:      "HBIDBSecretName",
			ConfigMapKey: "hbi.db.secret.name",
			DefaultValue: "host-inventory-db",
			Type:         reflect.String,
		},
		ElasticSearchSecretName: Parameter{
			SpecKey:      "ElasticSearchSecretName",
			ConfigMapKey: "elasticsearch.secret.name",
			DefaultValue: "xjoin-elasticsearch",
			Type:         reflect.String,
		},
		KafkaCluster: Parameter{
			SpecKey:      "KafkaCluster",
			ConfigMapKey: "kafka.cluster",
			DefaultValue: "xjoin-kafka-cluster",
			Type:         reflect.String,
		},
		StandardInterval: Parameter{
			ConfigMapKey: "standard.interval",
			DefaultValue: 120,
			Type:         reflect.Int,
		},
		ValidationInterval: Parameter{
			ConfigMapKey: "validation.interval",
			DefaultValue: 60 * 30,
			Type:         reflect.Int,
		},
		ValidationAttemptsThreshold: Parameter{
			ConfigMapKey: "validation.attempts.threshold",
			DefaultValue: 3,
			Type:         reflect.Int,
		},
		ValidationPercentageThreshold: Parameter{
			ConfigMapKey: "validation.percentage.threshold",
			DefaultValue: 5,
			Type:         reflect.Int,
		},
		ValidationInitInterval: Parameter{
			ConfigMapKey: "init.validation.interval",
			DefaultValue: 60,
			Type:         reflect.Int,
		},
		ValidationInitAttemptsThreshold: Parameter{
			ConfigMapKey: "init.validation.attempts.threshold",
			DefaultValue: 30,
			Type:         reflect.Int,
		},
		ValidationInitPercentageThreshold: Parameter{
			ConfigMapKey: "init.validation.percentage.threshold",
			DefaultValue: 5,
			Type:         reflect.Int,
		},
		ElasticSearchConnectorTemplate: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "elasticsearch.connector.config",
			DefaultValue: `{
				"tasks.max": "{{.TasksMax}}",
				"topics": "xjoin.inventory.{{.Version}}.public.hosts",
				"key.ignore": "false",
				"connection.url": "{{.ElasticSearchURL}}",
				"connection.username": "{{.ElasticSearchUsername}}",
				"connection.password": "{{.ElasticSearchPassword}}",
				"type.name": "_doc",
				"transforms": "valueToKey, extractKey, expandJSON, deleteIf, flattenList, flattenListString",
				"transforms.valueToKey.type":"org.apache.kafka.connect.transforms.ValueToKey",
				"transforms.valueToKey.fields":"id",
				"transforms.extractKey.type":"org.apache.kafka.connect.transforms.ExtractField$Key",
				"transforms.extractKey.field":"id",
				"transforms.expandJSON.type": "com.redhat.insights.expandjsonsmt.ExpandJSON$Value",
				"transforms.expandJSON.sourceFields": "tags",
				"transforms.deleteIf.type": "com.redhat.insights.deleteifsmt.DeleteIf$Value",
				"transforms.deleteIf.field": "__deleted",
				"transforms.deleteIf.value": "true",
				"transforms.flattenList.type": "com.redhat.insights.flattenlistsmt.FlattenList$Value",
				"transforms.flattenList.sourceField": "tags",
				"transforms.flattenList.outputField": "tags_structured",
				"transforms.flattenList.mode": "keys",
				"transforms.flattenList.keys": "namespace,key,value",
				"transforms.flattenListString.type": "com.redhat.insights.flattenlistsmt.FlattenList$Value",
				"transforms.flattenListString.sourceField": "tags",
				"transforms.flattenListString.outputField": "tags_string",
				"transforms.flattenListString.mode": "join",
				"transforms.flattenListString.delimiterJoin": "/",
				"transforms.flattenListString.encode": true,
				"behavior.on.null.values":"delete",
				"behavior.on.malformed.documents": "warn",
				"auto.create.indices.at.start": false,
				"schema.ignore": true,
				"max.in.flight.requests": {{.MaxInFlightRequests}},
				"errors.log.enable": {{.ErrorsLogEnable}},
				"errors.log.include.messages": true,
				"max.retries": {{.MaxRetries}},
				"retry.backoff.ms": {{.RetryBackoffMS}},
				"batch.size": {{.BatchSize}},
				"max.buffered.records": {{.MaxBufferedRecords}},
				"linger.ms": {{.LingerMS}}
			}`,
		},
		ElasticSearchURL: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.elasticSearch,
			SecretKey:    "url",
			DefaultValue: "http://localhost:9200",
		},
		ElasticSearchUsername: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.elasticSearch,
			SecretKey:    "username",
			DefaultValue: "xjoin",
		},
		ElasticSearchPassword: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.elasticSearch,
			SecretKey:    "password",
			DefaultValue: "xjoin1337",
		},
		ElasticSearchTasksMax: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "elasticsearch.connector.tasks.max",
			DefaultValue: 1,
		},
		ElasticSearchMaxInFlightRequests: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "elasticsearch.connector.max.in.flight.requests",
			DefaultValue: 1,
		},
		ElasticSearchErrorsLogEnable: Parameter{
			Type:         reflect.Bool,
			ConfigMapKey: "elasticsearch.connector.errors.log.enable",
			DefaultValue: true,
		},
		ElasticSearchMaxRetries: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "elasticsearch.connector.max.retries",
			DefaultValue: 8,
		},
		ElasticSearchRetryBackoffMS: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "elasticsearch.connector.retry.backoff.ms",
			DefaultValue: 100,
		},
		ElasticSearchBatchSize: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "elasticsearch.connector.batch.size",
			DefaultValue: 100,
		},
		ElasticSearchMaxBufferedRecords: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "elasticsearch.connector.max.buffered.records",
			DefaultValue: 500,
		},
		ElasticSearchLingerMS: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "elasticsearch.connector.linger.ms",
			DefaultValue: 100,
		},
		DebeziumTemplate: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "debezium.connector.config",
			DefaultValue: `{
				"tasks.max": "{{.TasksMax}}",
				"database.hostname": "{{.DBHostname}}",
				"database.port": "{{.DBPort}}",
				"database.user": "{{.DBUser}}",
				"database.password": "{{.DBPassword}}",
				"database.dbname": "{{.DBName}}",
				"database.server.name": "{{.ResourceNamePrefix}}.{{.Version}}",
				"table.whitelist": "public.hosts",
				"plugin.name": "pgoutput",
				"transforms": "unwrap",
				"transforms.unwrap.type": "io.debezium.transforms.ExtractNewRecordState",
				"transforms.unwrap.delete.handling.mode": "rewrite",
				"errors.log.enable": {{.ErrorsLogEnable}},
				"errors.log.include.messages": true,
				"slot.name": "{{.ReplicationSlotName}}",
				"max.queue.size": {{.QueueSize}},
				"max.batch.size": {{.MaxBatchSize}},
				"poll.interval.ms": {{.PollIntervalMS}}
			}`,
		},
		DebeziumTasksMax: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "debezium.connector.tasks.max",
			DefaultValue: 1,
		},
		DebeziumMaxBatchSize: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "debezium.connector.max.batch.size",
			DefaultValue: 10,
		},
		DebeziumQueueSize: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "debezium.connector.max.queue.size",
			DefaultValue: 1000,
		},
		DebeziumPollIntervalMS: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "debezium.connector.poll.interval.ms",
			DefaultValue: 100,
		},
		DebeziumErrorsLogEnable: Parameter{
			Type:         reflect.Bool,
			ConfigMapKey: "debezium.connector.errors.log.enable",
			DefaultValue: true,
		},
		HBIDBName: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.hbiDB,
			SecretKey:    "db.name",
			DefaultValue: "insights",
		},
		HBIDBHost: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.hbiDB,
			SecretKey:    "db.host",
			DefaultValue: "inventory-db",
		},
		HBIDBPort: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.hbiDB,
			SecretKey:    "db.port",
			DefaultValue: "5432",
		},
		HBIDBUser: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.hbiDB,
			SecretKey:    "db.user",
			DefaultValue: "insights",
		},
		HBIDBPassword: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.hbiDB,
			SecretKey:    "db.password",
			DefaultValue: "insights",
		},
		KafkaTopicPartitions: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "kafka.topic.partitions",
			DefaultValue: 1,
		},
		KafkaTopicReplicas: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "kafka.topic.replicas",
			DefaultValue: 1,
		},
	}
}
