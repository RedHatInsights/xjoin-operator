package config

const defaultConnectCluster = "xjoin-kafka-connect-strimzi"
const defaultKafkaCluster = "xjoin-kafka-cluster"
const defaultElasticSearchURL = "http://xjoin-elasticsearch-es-default:9200"
const defaultElasticSearchUsername = "xjoin"
const defaultElasticSearchPassword = "xjoin1337"

const defaultDebeziumConnectorTemplate = `{
    "tasks.max": "1",
    "database.hostname": "{{.DBHostname}}",
    "database.port": "{{.DBPort}}",
	"database.user": "{{.DBUser}}",
	"database.password": "{{.DBPassword}}",
	"database.dbname": "{{.DBName}}",
	"database.server.name": "xjoin.inventory.{{.Version}}",
	"table.whitelist": "public.hosts",
	"plugin.name": "pgoutput",
	"transforms": "unwrap",
	"transforms.unwrap.type": "io.debezium.transforms.ExtractNewRecordState",
	"transforms.unwrap.delete.handling.mode": "rewrite",
	"errors.log.enable": true,
	"errors.log.include.messages": true,
	"slot.name": "xjoin_inventory_{{.Version}}",
	"max.queue.size": 1000,
	"max.batch.size": 10,
	"poll.interval.ms": 100
}`
const defaultElasticSearchConnectorTemplate = `{
    "tasks.max": "50",
    "topics": "xjoin.inventory.{{.Version}}.public.hosts",
    "key.ignore": "false",
    "connection.url": "{{.ElasticSearchURL}}",
	"connection.username": "{{.ElasticSearchUsername}}",
	"connection.password": "{{.ElasticSearchPassword}}",
    "type.name": "_doc",
    "transforms": "valueToKey, extractKey, expandJSON, deleteIf, flattenList, flattenListString, renameTopic",
    "transforms.valueToKey.type":"org.apache.kafka.connect.transforms.ValueToKey",
    "transforms.valueToKey.fields":"id",
    "transforms.extractKey.type":"org.apache.kafka.connect.transforms.ExtractField$Key",
    "transforms.extractKey.field":"id",
    "transforms.expandJSON.type": "com.redhat.insights.expandjsonsmt.ExpandJSON$Value",
    "transforms.expandJSON.sourceFields": "tags",
    "transforms.deleteIf.type": "com.redhat.insights.deleteifsmt.DeleteIf$Value",
    "transforms.deleteIf.field": "__deleted",
    "transforms.deleteIf.value": "true",
    "transforms.renameTopic.type":"org.apache.kafka.connect.transforms.RegexRouter",
    "transforms.renameTopic.regex":"xjoin\\.inventory\\.{{.Version}}.public\\.hosts",
    "transforms.renameTopic.replacement":"xjoin.inventory.hosts.{{.Version}}",
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
    "max.in.flight.requests": 1,
    "errors.log.enable": true,
    "errors.log.include.messages": true,
    "max.retries": 8,
    "retry.backoff.ms": 100,
    "batch.size": 100,
    "max.buffered.records": 500,
    "linger.ms": 100
}`

const defaultStandardInterval int64 = 120
const defaultConnectorTasksMax int64 = 16
const defaultConnectorBatchSize int64 = 100
const defaultConnectorMaxAge int64 = 45

var defaultValidationConfig = ValidationConfiguration{
	Interval:            60 * 30,
	AttemptsThreshold:   3,
	PercentageThreshold: 5,
}

var defaultValidationConfigInit = ValidationConfiguration{
	Interval:            60,
	AttemptsThreshold:   30,
	PercentageThreshold: 5,
}
