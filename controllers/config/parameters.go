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
	Ephemeral                            Parameter
	ResourceNamePrefix                   Parameter
	ConnectCluster                       Parameter
	ConnectClusterNamespace              Parameter
	KafkaCluster                         Parameter
	KafkaClusterNamespace                Parameter
	ConfigMapVersion                     Parameter
	StandardInterval                     Parameter
	ValidationInterval                   Parameter
	ValidationAttemptsThreshold          Parameter
	ValidationPercentageThreshold        Parameter
	ValidationInitInterval               Parameter
	ValidationInitAttemptsThreshold      Parameter
	ValidationInitPercentageThreshold    Parameter
	ElasticSearchConnectorTemplate       Parameter
	ElasticSearchURL                     Parameter
	ElasticSearchUsername                Parameter
	ElasticSearchPassword                Parameter
	ElasticSearchTasksMax                Parameter
	ElasticSearchMaxInFlightRequests     Parameter
	ElasticSearchErrorsLogEnable         Parameter
	ElasticSearchMaxRetries              Parameter
	ElasticSearchRetryBackoffMS          Parameter
	ElasticSearchBatchSize               Parameter
	ElasticSearchMaxBufferedRecords      Parameter
	ElasticSearchLingerMS                Parameter
	ElasticSearchSecretName              Parameter
	ElasticSearchNamespace               Parameter
	ElasticSearchSecretVersion           Parameter
	ElasticSearchPipelineTemplate        Parameter
	ElasticSearchIndexReplicas           Parameter
	ElasticSearchIndexShards             Parameter
	ElasticSearchIndexTemplate           Parameter
	DebeziumTemplate                     Parameter
	DebeziumTasksMax                     Parameter
	DebeziumMaxBatchSize                 Parameter
	DebeziumQueueSize                    Parameter
	DebeziumPollIntervalMS               Parameter
	DebeziumErrorsLogEnable              Parameter
	HBIDBName                            Parameter
	HBIDBHost                            Parameter
	HBIDBPort                            Parameter
	HBIDBUser                            Parameter
	HBIDBPassword                        Parameter
	HBIDBSSLMode                         Parameter
	HBIDBSSLRootCert                     Parameter
	HBIDBSecretName                      Parameter
	HBIDBSecretVersion                   Parameter
	KafkaTopicPartitions                 Parameter
	KafkaTopicReplicas                   Parameter
	KafkaTopicCleanupPolicy              Parameter
	KafkaTopicMinCompactionLagMS         Parameter
	KafkaTopicRetentionBytes             Parameter
	KafkaTopicRetentionMS                Parameter
	JenkinsManagedVersion                Parameter
	FullValidationNumThreads             Parameter
	FullValidationChunkSize              Parameter
	FullValidationEnabled                Parameter
	ValidationPeriodMinutes              Parameter
	ValidationLagCompensationSeconds     Parameter
	KafkaTopicMessageBytes               Parameter
	KafkaTopicCreationTimeout            Parameter
	KafkaConnectReconcileIntervalSeconds Parameter
	ManagedKafka                         Parameter
	ManagedKafkaSecretName               Parameter
	ManagedKafkaSecretNamespace          Parameter
}

func NewXJoinConfiguration() Parameters {
	return Parameters{
		Ephemeral: Parameter{
			SpecKey:      "Ephemeral",
			DefaultValue: false,
			Type:         reflect.Bool,
		},
		JenkinsManagedVersion: Parameter{
			DefaultValue: "v1.160",
			Type:         reflect.String,
			ConfigMapKey: "jenkins.managed.version",
		},
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
			DefaultValue: "connect",
			Type:         reflect.String,
		},
		ConnectClusterNamespace: Parameter{
			SpecKey:      "ConnectClusterNamespace",
			ConfigMapKey: "connect.cluster.namespace",
			DefaultValue: "test",
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
		ElasticSearchNamespace: Parameter{
			SpecKey:      "ElasticSearchNamespace",
			ConfigMapKey: "elasticsearch.namespace",
			DefaultValue: "test",
			Type:         reflect.String,
		},
		KafkaCluster: Parameter{
			SpecKey:      "KafkaCluster",
			ConfigMapKey: "kafka.cluster",
			DefaultValue: "kafka",
			Type:         reflect.String,
		},
		KafkaClusterNamespace: Parameter{
			SpecKey:      "KafkaClusterNamespace",
			ConfigMapKey: "kafka.cluster.namespace",
			DefaultValue: "test",
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
		ElasticSearchIndexShards: Parameter{
			DefaultValue: 3,
			Type:         reflect.Int,
			ConfigMapKey: "elasticsearch.index.shards",
		},
		ElasticSearchIndexReplicas: Parameter{
			DefaultValue: 1,
			Type:         reflect.Int,
			ConfigMapKey: "elasticsearch.index.replicas",
		},
		ElasticSearchIndexTemplate: Parameter{
			DefaultValue: `
				{
				  "settings": {
					"index": {
					  "number_of_shards": "{{.ElasticSearchIndexShards}}",
					  "number_of_replicas": "{{.ElasticSearchIndexReplicas}}",
					  "default_pipeline": "{{.ElasticSearchPipeline}}",
					  "max_result_window": 100000
					},
					"analysis": {
					  "normalizer": {
						"case_insensitive": {
						  "filter": "lowercase"
						}
					  }
					}
				  },
				  "mappings": {
					"dynamic": false,
					"properties": {
					  "ingest_timestamp": {"type": "date"},
					  "id": { "type": "keyword" },
					  "account": { "type": "keyword" },
					  "org_id": { "type": "keyword" },
					  "display_name": {
						"type": "keyword",
						"fields": {
						  "lowercase": {
							"type": "keyword",
							"normalizer": "case_insensitive"
						  }
						}
					  },
					  "created_on": { "type": "date_nanos" },
					  "modified_on": { "type": "date_nanos" },
					  "stale_timestamp": { "type": "date_nanos" },
					  "ansible_host": { "type": "keyword" },
					  "canonical_facts": {
						"type": "object",
						"properties": {
						  "fqdn": {
							"type": "keyword",
							"fields": {
							  "lowercase": {
								"type": "keyword",
								"normalizer": "case_insensitive"
							  }
							}
						  },
						  "insights_id": { "type": "keyword"},
						  "satellite_id": { "type": "keyword"},
						  "provider_type": { "type": "keyword"},
						  "provider_id": { "type": "keyword"}
						}
					  },
					  "system_profile_facts": {
						"type": "object",
						"properties": {
						  "arch": { "type": "keyword" },
						  "os_release": { "type": "keyword" },
						  "os_kernel_version": { "type": "keyword"},
						  "infrastructure_type": { "type": "keyword" },
						  "infrastructure_vendor": { "type": "keyword" },
						  "sap_system": { "type": "boolean" },
						  "sap_sids": { "type": "keyword" },
						  "owner_id": { "type": "keyword"},
						  "insights_client_version": { "type": "keyword" },
						  "rhc_client_id": { "type": "keyword" },
						  "is_marketplace": { "type": "boolean" },
						  "operating_system": {
							"type": "object",
							"properties": {
							  "major": {"type": "byte"},
							  "minor": {"type": "byte"},
							  "name": {"type": "keyword"}
							}
						  },
						  "ansible": {
							"type": "object",
							"properties": {
							  "controller_version": {"type": "keyword"},
							  "hub_version": {"type": "keyword"},
							  "catalog_worker_version": {"type": "keyword"},
							  "sso_version": {"type": "keyword"}
							}
						  },
						  "host_type": { "type": "keyword" }
						}
					  },
					  "tags_structured": {
						"type": "nested",
						"properties": {
						  "namespace": {
							"type": "keyword",
							"null_value": "$$_XJOIN_SEARCH_NULL_VALUE"
						  },
						  "key": { "type": "keyword" },
						  "value": {
							"type": "keyword",
							"null_value": "$$_XJOIN_SEARCH_NULL_VALUE"
						  }
						}
					  },
					  "tags_string": {
						"type": "keyword"
					  },
					  "tags_search": {
						"type": "keyword"
					  },
					  "tags_search_combined": {
						"type": "keyword"
					  },
					  "per_reporter_staleness_flat": {
						"type": "nested",
						"properties": {
						  "reporter": {
							"type": "keyword"
						  },
						  "last_check_in": {
							"type": "keyword"
						  },
						  "stale_timestamp": {
							"type": "keyword"
						  },
						  "check_in_succeeded": {
							"type": "boolean"
						  }
						}
					  }
					}
				  }}`,
			Type:         reflect.String,
			ConfigMapKey: "elasticsearch.index.template",
			SpecKey:      "ElasticSearchIndexTemplate",
		},
		ElasticSearchConnectorTemplate: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "elasticsearch.connector.config",
			DefaultValue: `{
				"tasks.max": "{{.ElasticSearchTasksMax}}",
				"topics": "{{.Topic}}",
				"key.ignore": "false",
				"connection.url": "{{.ElasticSearchURL}}",
				{{if .ElasticSearchUsername}}"connection.username": "{{.ElasticSearchUsername}}",{{end}}
				{{if .ElasticSearchPassword}}"connection.password": "{{.ElasticSearchPassword}}",{{end}}
				"type.name": "_doc",
				"transforms": "valueToKey, extractKey, expandJSON, expandPRSJSON, deleteIf, flattenList, flattenListString, flattenPRS, renameTopic",
				"transforms.valueToKey.type":"org.apache.kafka.connect.transforms.ValueToKey",
				"transforms.valueToKey.fields":"id",
				"transforms.extractKey.type":"org.apache.kafka.connect.transforms.ExtractField$Key",
				"transforms.extractKey.field":"id",
				"transforms.expandJSON.type": "com.redhat.insights.expandjsonsmt.ExpandJSON$Value",
				"transforms.expandJSON.sourceFields": "tags",
				"transforms.expandPRSJSON.type": "com.redhat.insights.expandjsonsmt.ExpandJSON$Value",
				"transforms.expandPRSJSON.sourceFields": "per_reporter_staleness",
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
				"transforms.flattenPRS.type": "com.redhat.insights.flattenlistsmt.FlattenList$Value",
				"transforms.flattenPRS.sourceField": "per_reporter_staleness",
				"transforms.flattenPRS.outputField": "per_reporter_staleness_flat",
				"transforms.flattenPRS.mode": "object",
				"transforms.flattenPRS.rootKey": "reporter",
				"transforms.renameTopic.type": "org.apache.kafka.connect.transforms.RegexRouter",
				"transforms.renameTopic.regex": "{{.Topic}}",
				"transforms.renameTopic.replacement": "{{.RenameTopicReplacement}}",
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
			SecretKey:    []string{"endpoint"},
			DefaultValue: "http://localhost:9200",
		},
		ElasticSearchUsername: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.elasticSearch,
			SecretKey:    []string{"username"},
			DefaultValue: "xjoin",
		},
		ElasticSearchPassword: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.elasticSearch,
			SecretKey:    []string{"password"},
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
						"source": "ctx.tags_search = ctx.tags_structured.stream().map(t -> { StringBuilder builder = new StringBuilder(); if (t.namespace != null && t.namespace != 'null') { builder.append(t.namespace); } builder.append('/'); builder.append(t.key); builder.append('='); if (t.value != null) { builder.append(t.value); } return builder.toString() }).collect(Collectors.toList()); ctx.tags_search_combined = ctx.tags_search.stream().map(t -> {return t + 'c6509b6d-9646-4122-a16c-f536660c22ee' + t.toLowerCase()}).collect(Collectors.toList());"
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
				"database.sslmode": "{{.HBIDBSSLMode}}",
				"database.sslrootcert": "{{.HBIDBSSLRootCert}}",
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
			SecretKey:    []string{"db.name", "name"},
			DefaultValue: "insights",
		},
		HBIDBHost: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.hbiDB,
			SecretKey:    []string{"db.host", "hostname"},
			DefaultValue: "inventory-db",
		},
		HBIDBPort: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.hbiDB,
			SecretKey:    []string{"db.port", "port"},
			DefaultValue: "5432",
		},
		HBIDBUser: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.hbiDB,
			SecretKey:    []string{"db.user", "username"},
			DefaultValue: "insights",
		},
		HBIDBPassword: Parameter{
			Type:         reflect.String,
			Secret:       secretTypes.hbiDB,
			SecretKey:    []string{"db.password", "password"},
			DefaultValue: "insights",
		},
		HBIDBSSLMode: Parameter{
			Type:         reflect.String,
			DefaultValue: "disable",
			ConfigMapKey: "hbi.db.ssl.mode",
		},
		KafkaTopicPartitions: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "kafka.topic.partitions",
			DefaultValue: 1,
		},
		KafkaTopicReplicas: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "kafka.topic.replicas",
			DefaultValue: 3,
		},
		KafkaTopicCleanupPolicy: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "kafka.topic.cleanup.policy",
			DefaultValue: "compact,delete",
		},
		KafkaTopicMinCompactionLagMS: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "kafka.topic.min.compaction.lag.ms",
			DefaultValue: "3600000",
		},
		KafkaTopicRetentionBytes: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "kafka.topic.retention.bytes",
			DefaultValue: "5368709120",
		},
		KafkaTopicRetentionMS: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "kafka.topic.retention.ms",
			DefaultValue: "2678400001",
		},
		KafkaTopicMessageBytes: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "kafka.topic.max.message.bytes",
			DefaultValue: "2097176",
		},
		FullValidationChunkSize: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "full.validation.chunk.size",
			DefaultValue: 2000,
		},
		FullValidationNumThreads: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "full.validation.num.threads",
			DefaultValue: 20,
		},
		FullValidationEnabled: Parameter{
			Type:         reflect.Bool,
			ConfigMapKey: "full.validation.enabled",
			DefaultValue: true,
		},
		ValidationPeriodMinutes: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "validation.period.minutes",
			DefaultValue: 60,
		},
		ValidationLagCompensationSeconds: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "validation.lag.compensation.seconds",
			DefaultValue: 120,
		},
		KafkaTopicCreationTimeout: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "kafka.topic.creation.timeout",
			DefaultValue: 300,
		},
		KafkaConnectReconcileIntervalSeconds: Parameter{
			Type:         reflect.Int,
			ConfigMapKey: "kafka.connect.reconcile.interval.seconds",
			DefaultValue: 120,
		},
		HBIDBSSLRootCert: Parameter{
			Type:         reflect.String,
			ConfigMapKey: "hbi.db.ssl.root.cert",
			DefaultValue: "/opt/kafka/external-configuration/rds-client-ca/rds-cacert",
		},
		ManagedKafka: Parameter{
			Type:         reflect.Bool,
			DefaultValue: false,
			SpecKey:      "ManagedKafka",
		},
		ManagedKafkaSecretName: Parameter{
			Type:         reflect.String,
			DefaultValue: "ephem-managed-kafka",
			SpecKey:      "ManagedKafkaSecretName",
		},
		ManagedKafkaSecretNamespace: Parameter{
			Type:         reflect.String,
			DefaultValue: "xjoin",
			SpecKey:      "ManagedKafkaSecretNamespace",
		},
	}
}
