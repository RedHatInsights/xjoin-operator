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
	KafkaClusterNamespace             Parameter
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
	ElasticSearchSecretName           Parameter
	ElasticSearchSecretVersion        Parameter
	ElasticSearchPipelineTemplate     Parameter
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
	HBIDBSecretVersion                Parameter
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
		KafkaClusterNamespace: Parameter{
			SpecKey:      "KafkaClusterNamespace",
			ConfigMapKey: "kafka.cluster.namespace",
			DefaultValue: "xjoin-operator-project",
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
			DefaultValue: 1,
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
			DefaultValue: 1,
			Type:         reflect.Int,
		},
		ValidationInitPercentageThreshold: Parameter{
			ConfigMapKey: "init.validation.percentage.threshold",
			DefaultValue: 5,
			Type:         reflect.Int,
		},
		ElasticSearchSecretVersion: Parameter{
			DefaultValue: "",
			Type:         reflect.String,
		},
		ElasticSearchConnectorTemplate: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "elasticsearch.connector.config",
			DefaultValue: `{
				"tasks.max": "{{.ElasticSearchTasksMax}}",
				"topics": "{{.ResourceNamePrefix}}.{{.Version}}.public.hosts",
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
				"max.in.flight.requests": {{.ElasticSearchMaxInFlightRequests}},
				"errors.log.enable": {{.ElasticSearchErrorsLogEnable}},
				"errors.log.include.messages": true,
				"max.retries": {{.ElasticSearchMaxRetries}},
				"retry.backoff.ms": {{.ElasticSearchRetryBackoffMS}},
				"batch.size": {{.ElasticSearchBatchSize}},
				"max.buffered.records": {{.ElasticSearchMaxBufferedRecords}},
				"linger.ms": {{.ElasticSearchLingerMS}}
			}`,
		},
		ElasticSearchURL: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.elasticSearch,
			SecretKey:    "endpoint",
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
		ElasticSearchPipelineTemplate: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "elasticsearch.pipeline.template",
			DefaultValue: `{
				"description" : "Ingest pipeline for {{.ResourceNamePrefix}}",
				"processors" : [{
					"set": {
						"field": "ingest_timestamp",
						"value": "{{"{{"}}_ingest.timestamp{{"}}"}}"
					},
					"json" : {
						"if" : "ctx.system_profile_facts != null",
						"field" : "system_profile_facts"
					}
				}, {
					"json" : {
						"if" : "ctx.canonical_facts != null",
						"field" : "canonical_facts"
					}
				}, {
					"json" : {
						"if" : "ctx.facts != null",
						"field" : "facts"
					}
				}, {
					"script": {
						"lang": "painless",
						"if": "ctx.tags_structured != null",
						"source": "ctx.tags_search = ctx.tags_structured.stream().map(t -> { StringBuilder builder = new StringBuilder(); if (t.namespace != null && t.namespace != 'null') { builder.append(t.namespace); } builder.append('/'); builder.append(t.key); builder.append('='); if (t.value != null) { builder.append(t.value); } return builder.toString() }).collect(Collectors.toList())"
					}
				}]
			}`,
		},
		DebeziumTemplate: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "debezium.connector.config",
			DefaultValue: `{
				"tasks.max": "{{.DebeziumTasksMax}}",
				"database.hostname": "{{.HBIDBHost}}",
				"database.port": "{{.HBIDBPort}}",
				"database.user": "{{.HBIDBUser}}",
				"database.password": "{{.HBIDBPassword}}",
				"database.dbname": "{{.HBIDBName}}",
				"database.server.name": "{{.ResourceNamePrefix}}.{{.Version}}",
				"table.whitelist": "public.hosts",
				"plugin.name": "pgoutput",
				"transforms": "unwrap",
				"transforms.unwrap.type": "io.debezium.transforms.ExtractNewRecordState",
				"transforms.unwrap.delete.handling.mode": "rewrite",
				"errors.log.enable": {{.DebeziumErrorsLogEnable}},
				"errors.log.include.messages": true,
				"slot.name": "{{.ReplicationSlotName}}",
				"max.queue.size": {{.DebeziumQueueSize}},
				"max.batch.size": {{.DebeziumMaxBatchSize}},
				"poll.interval.ms": {{.DebeziumPollIntervalMS}}
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
		HBIDBSecretVersion: Parameter{
			DefaultValue: "",
			Type:         reflect.String,
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
