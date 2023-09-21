package test

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	"gopkg.in/h2non/gock.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/record"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipeline operations", func() {
	var i *Iteration

	BeforeEach(func() {
		iteration, err := Before()
		Expect(err).ToNot(HaveOccurred())
		i = iteration
	})

	AfterEach(func() {
		err := After(i)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("New -> InitialSync", func() {
		It("Creates a connector, ES Index, and topic for a new pipeline", func() {
			err := i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			_, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())

			pipeline, err := i.GetPipeline()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			dbConnector, err := i.KafkaClient.GetConnector(
				i.KafkaConnectors.DebeziumConnectorName(pipeline.Status.PipelineVersion), i.KafkaClient.ConnectNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(dbConnector.GetName()).To(Equal(ResourceNamePrefix + ".db." + pipeline.Status.PipelineVersion))
			dbConnectorSpec := dbConnector.Object["spec"].(map[string]interface{})
			Expect(dbConnectorSpec["class"]).To(Equal("io.debezium.connector.postgresql.PostgresConnector"))
			Expect(dbConnectorSpec["pause"]).To(Equal(false))
			dbConnectorConfig := dbConnectorSpec["config"].(map[string]interface{})
			Expect(dbConnectorConfig["database.dbname"]).To(Equal("test"))
			Expect(dbConnectorConfig["database.password"]).To(Equal("insights"))
			Expect(dbConnectorConfig["database.port"]).To(Equal("5432"))
			Expect(dbConnectorConfig["tasks.max"]).To(Equal("1"))
			Expect(dbConnectorConfig["database.user"]).To(Equal("insights"))
			Expect(dbConnectorConfig["max.batch.size"]).To(Equal(int64(10)))
			Expect(dbConnectorConfig["plugin.name"]).To(Equal("pgoutput"))
			Expect(dbConnectorConfig["transforms"]).To(Equal("unwrap"))
			Expect(dbConnectorConfig["transforms.unwrap.delete.handling.mode"]).To(Equal("rewrite"))
			Expect(dbConnectorConfig["database.hostname"]).To(Equal("host-inventory-db.test.svc"))
			Expect(dbConnectorConfig["errors.log.include.messages"]).To(Equal(true))
			Expect(dbConnectorConfig["max.queue.size"]).To(Equal(int64(1000)))
			Expect(dbConnectorConfig["poll.interval.ms"]).To(Equal(int64(100)))
			Expect(dbConnectorConfig["slot.name"]).To(Equal(ResourceNamePrefix + "_" + pipeline.Status.PipelineVersion))
			Expect(dbConnectorConfig["table.whitelist"]).To(Equal("public.hosts"))
			Expect(dbConnectorConfig["database.server.name"]).To(Equal(ResourceNamePrefix + "." + pipeline.Status.PipelineVersion))
			Expect(dbConnectorConfig["errors.log.enable"]).To(Equal(true))
			Expect(dbConnectorConfig["transforms.unwrap.type"]).To(Equal("io.debezium.transforms.ExtractNewRecordState"))

			esConnector, err := i.KafkaClient.GetConnector(
				i.KafkaConnectors.ESConnectorName(pipeline.Status.PipelineVersion), i.KafkaClient.ConnectNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(esConnector.GetName()).To(Equal(ResourceNamePrefix + ".es." + pipeline.Status.PipelineVersion))
			esConnectorSpec := esConnector.Object["spec"].(map[string]interface{})
			Expect(esConnectorSpec["class"]).To(Equal("io.confluent.connect.elasticsearch.ElasticsearchSinkConnector"))
			Expect(esConnectorSpec["pause"]).To(Equal(false))
			esConnectorConfig := esConnectorSpec["config"].(map[string]interface{})
			Expect(esConnectorConfig["max.buffered.records"]).To(Equal(int64(500)))
			Expect(esConnectorConfig["transforms.deleteIf.type"]).To(Equal("com.redhat.insights.deleteifsmt.DeleteIf$Value"))
			Expect(esConnectorConfig["transforms.flattenList.sourceField"]).To(Equal("tags"))
			Expect(esConnectorConfig["errors.log.include.messages"]).To(Equal(true))
			Expect(esConnectorConfig["linger.ms"]).To(Equal(int64(100)))
			Expect(esConnectorConfig["retry.backoff.ms"]).To(Equal(int64(100)))
			Expect(esConnectorConfig["tasks.max"]).To(Equal("1"))
			Expect(esConnectorConfig["topics"]).To(Equal(ResourceNamePrefix + "." + pipeline.Status.PipelineVersion + ".public.hosts"))
			Expect(esConnectorConfig["transforms.expandJSON.sourceFields"]).To(Equal("tags"))
			Expect(esConnectorConfig["transforms.flattenListString.sourceField"]).To(Equal("tags"))
			Expect(esConnectorConfig["auto.create.indices.at.start"]).To(Equal(false))
			Expect(esConnectorConfig["behavior.on.null.values"]).To(Equal("delete"))
			Expect(esConnectorConfig["connection.url"]).To(Equal("http://xjoin-elasticsearch-es-default.test.svc:9200"))
			Expect(esConnectorConfig["errors.log.enable"]).To(Equal(true))
			Expect(esConnectorConfig["max.retries"]).To(Equal(int64(8)))
			Expect(esConnectorConfig["transforms.deleteIf.field"]).To(Equal("__deleted"))
			Expect(esConnectorConfig["transforms.extractKey.field"]).To(Equal("id"))
			Expect(esConnectorConfig["transforms.flattenListString.type"]).To(Equal("com.redhat.insights.flattenlistsmt.FlattenList$Value"))
			Expect(esConnectorConfig["transforms.valueToKey.type"]).To(Equal("org.apache.kafka.connect.transforms.ValueToKey"))
			Expect(esConnectorConfig["transforms"]).To(Equal("valueToKey, extractKey, expandJSON, expandPRSJSON, deleteIf, flattenList, flattenListString, flattenPRS, renameTopic"))
			Expect(esConnectorConfig["transforms.flattenList.mode"]).To(Equal("keys"))
			Expect(esConnectorConfig["transforms.flattenListString.encode"]).To(Equal(true))
			Expect(esConnectorConfig["transforms.flattenListString.outputField"]).To(Equal("tags_string"))
			Expect(esConnectorConfig["transforms.renameTopic.type"]).To(Equal("org.apache.kafka.connect.transforms.RegexRouter"))
			Expect(esConnectorConfig["transforms.renameTopic.regex"]).To(Equal(ResourceNamePrefix + "." + pipeline.Status.PipelineVersion + ".public.hosts"))
			Expect(esConnectorConfig["transforms.renameTopic.replacement"]).To(Equal(ResourceNamePrefix + "." + pipeline.Status.PipelineVersion))
			Expect(esConnectorConfig["type.name"]).To(Equal("_doc"))
			Expect(esConnectorConfig["key.ignore"]).To(Equal("false"))
			Expect(esConnectorConfig["transforms.valueToKey.fields"]).To(Equal("id"))
			Expect(esConnectorConfig["behavior.on.malformed.documents"]).To(Equal("warn"))
			Expect(esConnectorConfig["connection.username"]).To(Equal("test"))
			Expect(esConnectorConfig["schema.ignore"]).To(Equal(true))
			Expect(esConnectorConfig["transforms.expandJSON.type"]).To(Equal("com.redhat.insights.expandjsonsmt.ExpandJSON$Value"))
			Expect(esConnectorConfig["transforms.flattenList.outputField"]).To(Equal("tags_structured"))
			Expect(esConnectorConfig["transforms.flattenList.type"]).To(Equal("com.redhat.insights.flattenlistsmt.FlattenList$Value"))
			Expect(esConnectorConfig["transforms.flattenListString.delimiterJoin"]).To(Equal("/"))
			Expect(esConnectorConfig["batch.size"]).To(Equal(int64(100)))
			Expect(esConnectorConfig["transforms.deleteIf.value"]).To(Equal("true"))
			Expect(esConnectorConfig["transforms.extractKey.type"]).To(Equal("org.apache.kafka.connect.transforms.ExtractField$Key"))
			Expect(esConnectorConfig["transforms.flattenList.keys"]).To(Equal("namespace,key,value"))
			Expect(esConnectorConfig["transforms.flattenListString.mode"]).To(Equal("join"))
			Expect(esConnectorConfig["max.in.flight.requests"]).To(Equal(int64(1)))

			exists, err := i.EsClient.IndexExists(i.EsClient.ESIndexName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			aliases, err := i.EsClient.GetCurrentIndicesWithAlias(*pipeline.Spec.ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(aliases).To(BeEmpty())

			topics, err := i.KafkaTopics.ListTopicNamesForPipelineVersion(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(topics).To(ContainElement(i.KafkaTopics.TopicName(pipeline.Status.PipelineVersion)))
		})

		It("Creates ESPipeline for new xjoin pipeline", func() {
			err := i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			_, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())

			pipeline, err := i.GetPipeline()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			esPipeline, err := i.EsClient.GetESPipeline(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(esPipeline).ToNot(BeEmpty())
			Expect(esPipeline).To(HaveKey(i.EsClient.ESPipelineName(pipeline.Status.PipelineVersion)))
		})

		It("Considers configmap configuration", func() {
			err := i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())

			cm := map[string]string{
				"debezium.connector.tasks.max":                        "-1",
				"debezium.connector.max.batch.size":                   "-2",
				"debezium.connector.max.queue.size":                   "-3",
				"debezium.connector.poll.interval.ms":                 "-4",
				"debezium.connector.errors.log.enable":                "false",
				"elasticsearch.connector.tasks.max":                   "-5",
				"elasticsearch.connector.batch.size":                  "-6",
				"elasticsearch.connector.max.in.flight.requests":      "-7",
				"elasticsearch.connector.errors.log.enable":           "false",
				"elasticsearch.connector.errors.log.include.messages": "false",
				"elasticsearch.connector.max.retries":                 "-8",
				"elasticsearch.connector.retry.backoff.ms":            "-9",
				"elasticsearch.connector.max.buffered.records":        "-10",
				"elasticsearch.connector.linger.ms":                   "-11",
				"standard.interval":                                   "-12",
				"validation.percentage.threshold":                     "-13",
				"init.validation.percentage.threshold":                "-14",
				"validation.attempts.threshold":                       "-15",
				"init.validation.attempts.threshold":                  "-16",
				"validation.interval":                                 "-17",
				"init.validation.interval":                            "-18",
			}

			err = i.CreateConfigMap("xjoin", cm)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())

			pipeline, err := i.GetPipeline()
			Expect(err).ToNot(HaveOccurred())
			esConnector, err := i.KafkaClient.GetConnector(
				i.KafkaConnectors.ESConnectorName(pipeline.Status.PipelineVersion), i.KafkaClient.ConnectNamespace)

			Expect(err).ToNot(HaveOccurred())
			Expect(esConnector.GetName()).To(Equal(ResourceNamePrefix + ".es." + pipeline.Status.PipelineVersion))
			esConnectorSpec := esConnector.Object["spec"].(map[string]interface{})
			esConnectorConfig := esConnectorSpec["config"].(map[string]interface{})
			Expect(esConnectorConfig["tasks.max"]).To(Equal(cm["elasticsearch.connector.tasks.max"]))
			Expect(esConnectorConfig["topics"]).To(Equal(ResourceNamePrefix + "." + pipeline.Status.PipelineVersion + ".public.hosts"))
			val, err := StrToInt64(cm["elasticsearch.connector.batch.size"])
			Expect(err).ToNot(HaveOccurred())
			Expect(esConnectorConfig["batch.size"]).To(Equal(val))
			val, err = StrToInt64(cm["elasticsearch.connector.max.in.flight.requests"])
			Expect(err).ToNot(HaveOccurred())
			Expect(esConnectorConfig["max.in.flight.requests"]).To(Equal(val))
			boolVal, err := StrToBool(cm["elasticsearch.connector.errors.log.enable"])
			Expect(err).ToNot(HaveOccurred())
			Expect(esConnectorConfig["errors.log.enable"]).To(Equal(boolVal))
			val, err = StrToInt64(cm["elasticsearch.connector.max.retries"])
			Expect(err).ToNot(HaveOccurred())
			Expect(esConnectorConfig["max.retries"]).To(Equal(val))
			val, err = StrToInt64(cm["elasticsearch.connector.retry.backoff.ms"])
			Expect(err).ToNot(HaveOccurred())
			Expect(esConnectorConfig["retry.backoff.ms"]).To(Equal(val))
			val, err = StrToInt64(cm["elasticsearch.connector.max.buffered.records"])
			Expect(err).ToNot(HaveOccurred())
			Expect(esConnectorConfig["max.buffered.records"]).To(Equal(val))
			val, err = StrToInt64(cm["elasticsearch.connector.linger.ms"])
			Expect(err).ToNot(HaveOccurred())
			Expect(esConnectorConfig["linger.ms"]).To(Equal(val))

			dbConnector, err := i.KafkaClient.GetConnector(
				i.KafkaConnectors.DebeziumConnectorName(pipeline.Status.PipelineVersion), i.KafkaClient.ConnectNamespace)

			Expect(err).ToNot(HaveOccurred())
			Expect(dbConnector.GetName()).To(Equal(ResourceNamePrefix + ".db." + pipeline.Status.PipelineVersion))
			dbConnectorSpec := dbConnector.Object["spec"].(map[string]interface{})
			dbConnectorConfig := dbConnectorSpec["config"].(map[string]interface{})
			Expect(dbConnectorConfig["tasks.max"]).To(Equal(cm["debezium.connector.tasks.max"]))
			val, err = StrToInt64(cm["debezium.connector.max.batch.size"])
			Expect(err).ToNot(HaveOccurred())
			Expect(dbConnectorConfig["max.batch.size"]).To(Equal(val))
			val, err = StrToInt64(cm["debezium.connector.max.queue.size"])
			Expect(err).ToNot(HaveOccurred())
			Expect(dbConnectorConfig["max.queue.size"]).To(Equal(val))
			val, err = StrToInt64(cm["debezium.connector.poll.interval.ms"])
			Expect(err).ToNot(HaveOccurred())
			Expect(dbConnectorConfig["poll.interval.ms"]).To(Equal(val))
			boolVal, err = StrToBool(cm["debezium.connector.errors.log.enable"])
			Expect(err).ToNot(HaveOccurred())
			Expect(dbConnectorConfig["errors.log.enable"]).To(Equal(boolVal))
		})

		It("Considers db secret name configuration", func() {
			ctx, cancel := utils.DefaultContext()
			defer cancel()
			hbiDBSecret, err := k8sUtils.FetchSecret(
				Client, i.NamespacedName.Namespace, i.Parameters.HBIDBSecretName.String(), ctx)
			Expect(err).ToNot(HaveOccurred())
			err = Client.Delete(ctx, hbiDBSecret)
			Expect(err).ToNot(HaveOccurred())

			secretName := "test-hbi-db-secret"
			err = i.CreateDbSecret(secretName)
			Expect(err).ToNot(HaveOccurred())

			err = i.CreatePipeline(&xjoin.XJoinPipelineSpec{HBIDBSecretName: &secretName})
			Expect(err).ToNot(HaveOccurred())
			_, err = i.ReconcileXJoin() //this will fail if the secret is missing
			Expect(err).ToNot(HaveOccurred())
		})

		It("Considers es secret name configuration", func() {
			ctx, cancel := utils.DefaultContext()
			defer cancel()
			elasticSearchSecret, err := k8sUtils.FetchSecret(Client, i.NamespacedName.Namespace, i.Parameters.ElasticSearchSecretName.String(), ctx)
			Expect(err).ToNot(HaveOccurred())
			err = Client.Delete(ctx, elasticSearchSecret)
			Expect(err).ToNot(HaveOccurred())

			secretName := "test-elasticsearch-secret"
			err = i.CreateESSecret(secretName)
			Expect(err).ToNot(HaveOccurred())

			err = i.CreatePipeline(&xjoin.XJoinPipelineSpec{ElasticSearchSecretName: &secretName})
			Expect(err).ToNot(HaveOccurred())
			_, err = i.ReconcileXJoin() //this will fail if the secret is missing
			Expect(err).ToNot(HaveOccurred())
		})

		It("Removes stale connectors", func() {
			_, err := i.KafkaConnectors.CreateDebeziumConnector("1", false)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.KafkaConnectors.CreateDebeziumConnector("2", false)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.KafkaConnectors.CreateESConnector("1", false)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.KafkaConnectors.CreateESConnector("2", false)
			Expect(err).ToNot(HaveOccurred())

			connectors, err := i.KafkaClient.ListConnectors()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(connectors.Items)).To(Equal(4))

			err = i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			_, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())
			connectors, err = i.KafkaClient.ListConnectors()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(connectors.Items)).To(Equal(2))
		})

		It("Removes stale indices", func() {
			err := i.EsClient.CreateIndex("1")
			Expect(err).ToNot(HaveOccurred())
			err = i.EsClient.CreateIndex("2")
			Expect(err).ToNot(HaveOccurred())

			indices, err := i.EsClient.ListIndices()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(indices)).To(Equal(2))

			err = i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			_, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())
			indices, err = i.EsClient.ListIndices()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(indices)).To(Equal(1))
		})

		It("Removes stale topics", func() {
			_, err := i.KafkaTopics.CreateTopic("1", false)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.KafkaTopics.CreateTopic("2", false)
			Expect(err).ToNot(HaveOccurred())

			topics, err := i.KafkaTopics.ListTopicNamesForPrefix(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(topics)).To(Equal(2))

			err = i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			_, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())

			topics, err = i.KafkaTopics.ListTopicNamesForPrefix(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(topics)).To(Equal(1))
		})

		It("Removes stale replication slots", func() {
			prefix := "prefix.withadot"
			slot1 := database.ReplicationSlotPrefix(prefix) + "_1"
			slot2 := database.ReplicationSlotPrefix(prefix) + "_2"
			err := i.DbClient.RemoveReplicationSlotsForPrefix(prefix)
			Expect(err).ToNot(HaveOccurred())
			err = i.DbClient.CreateReplicationSlot(slot1)
			Expect(err).ToNot(HaveOccurred())
			err = i.DbClient.CreateReplicationSlot(slot2)
			Expect(err).ToNot(HaveOccurred())

			slots, err := i.DbClient.ListReplicationSlotsForPrefix(prefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(slots).To(ContainElements(slot1, slot2))

			err = i.CreatePipeline(&xjoin.XJoinPipelineSpec{ResourceNamePrefix: &prefix})
			Expect(err).ToNot(HaveOccurred())
			_, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())

			slots, err = i.DbClient.ListReplicationSlotsForPrefix(prefix)
			Expect(err).ToNot(HaveOccurred())

			Expect(slots).ToNot(ContainElements(slot1, slot2))
		})
	})

	Describe("InitialSync -> Valid", func() {
		It("Creates the elasticsearch alias", func() {
			err := i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			_, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())
			pipeline, err := i.GetPipeline()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			_, err = i.ReconcileValidation()
			Expect(err).ToNot(HaveOccurred())
			pipeline, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())

			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))
		})

		It("Triggers refresh if pipeline fails to become valid for too long", func() {
			err := i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())

			cm := map[string]string{
				"init.validation.attempts.threshold": "2",
			}

			err = i.CreateConfigMap("xjoin", cm)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())
			pipeline, err := i.GetPipeline()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			err = i.KafkaConnectors.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			hostId, err := i.InsertSimpleHost()
			Expect(err).ToNot(HaveOccurred())

			_, err = i.ReconcileValidation()
			Expect(err).ToNot(HaveOccurred())
			pipeline, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))

			_, err = i.ReconcileValidation()
			Expect(err).ToNot(HaveOccurred())
			pipeline, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_NEW))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))
			Expect(pipeline.Status.PipelineVersion).To(Equal(""))

			_, err = i.ReconcileValidation()
			Expect(err).ToNot(HaveOccurred())
			pipeline, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.PipelineVersion).ToNot(Equal(""))
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			err = i.IndexSimpleDocument(pipeline.Status.PipelineVersion, hostId)
			Expect(err).ToNot(HaveOccurred())

			_, err = i.ReconcileValidation()
			Expect(err).ToNot(HaveOccurred())
			pipeline, err = i.ReconcileXJoin()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))
		})

		It("Sets active resource names for a valid pipeline", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).To(Equal(i.EsClient.ESIndexName(pipeline.Status.PipelineVersion)))
			Expect(pipeline.Status.ActiveTopicName).To(Equal(i.KafkaTopics.TopicName(pipeline.Status.PipelineVersion)))
			Expect(pipeline.Status.ActiveAliasName).To(Equal(i.EsClient.AliasName()))
			Expect(pipeline.Status.ActiveDebeziumConnectorName).To(
				Equal(i.KafkaConnectors.DebeziumConnectorName(pipeline.Status.PipelineVersion)))
			Expect(pipeline.Status.ActiveESConnectorName).To(
				Equal(i.KafkaConnectors.ESConnectorName(pipeline.Status.PipelineVersion)))
		})
	})

	Describe("Invalid -> New", func() {
		Context("In a refresh", func() {
			It("Keeps the old table active until the new one is valid", func() {
				cm := map[string]string{
					"validation.attempts.threshold": "2",
				}
				err := i.CreateConfigMap("xjoin", cm)
				Expect(err).ToNot(HaveOccurred())

				pipeline, err := i.CreateValidPipeline()
				Expect(err).ToNot(HaveOccurred())
				activeIndex := pipeline.Status.ActiveIndexName

				err = i.KafkaConnectors.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
				Expect(err).ToNot(HaveOccurred())
				hostId, err := i.InsertSimpleHost()
				Expect(err).ToNot(HaveOccurred())

				pipeline, err = i.ExpectInvalidReconcile()
				Expect(err).ToNot(HaveOccurred())
				Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

				pipeline, err = i.ExpectNewReconcile()
				Expect(err).ToNot(HaveOccurred())
				Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

				pipeline, err = i.ExpectInitSyncUnknownReconcile()
				Expect(err).ToNot(HaveOccurred())
				Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

				err = i.IndexSimpleDocument(pipeline.Status.PipelineVersion, hostId)
				Expect(err).ToNot(HaveOccurred())

				pipeline, err = i.ExpectValidReconcile()
				Expect(err).ToNot(HaveOccurred())
				Expect(pipeline.Status.ActiveIndexName).ToNot(Equal(activeIndex))
			})
		})
	})

	Describe("Valid -> New", func() {
		It("Preserves active resource names during refresh", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			activeIndexName := i.EsClient.ESIndexName(pipeline.Status.PipelineVersion)
			activeTopicName := i.KafkaTopics.TopicName(pipeline.Status.PipelineVersion)
			activeAliasName := i.EsClient.AliasName()
			activeDebeziumConnectorName := i.KafkaConnectors.DebeziumConnectorName(pipeline.Status.PipelineVersion)
			activeESConnectorName := i.KafkaConnectors.ESConnectorName(pipeline.Status.PipelineVersion)

			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndexName))
			Expect(pipeline.Status.ActiveTopicName).To(Equal(activeTopicName))
			Expect(pipeline.Status.ActiveAliasName).To(Equal(activeAliasName))
			Expect(pipeline.Status.ActiveDebeziumConnectorName).To(Equal(activeDebeziumConnectorName))
			Expect(pipeline.Status.ActiveESConnectorName).To(Equal(activeESConnectorName))

			//trigger refresh with a new configmap
			validationPeriodMinutes := "1"
			cm := map[string]string{
				"validation.period.minutes": validationPeriodMinutes,
			}
			err = i.CreateConfigMap("xjoin", cm)
			Expect(err).ToNot(HaveOccurred())

			pipeline, err = i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndexName))
			Expect(pipeline.Status.ActiveTopicName).To(Equal(activeTopicName))
			Expect(pipeline.Status.ActiveAliasName).To(Equal(activeAliasName))
			Expect(pipeline.Status.ActiveDebeziumConnectorName).To(Equal(activeDebeziumConnectorName))
			Expect(pipeline.Status.ActiveESConnectorName).To(Equal(activeESConnectorName))
		})

		It("Triggers refresh if configmap is created", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			activeIndex := pipeline.Status.ActiveIndexName

			cm := map[string]string{
				"debezium.connector.errors.log.enable": "false",
			}
			err = i.CreateConfigMap("xjoin", cm)
			Expect(err).ToNot(HaveOccurred())

			pipeline, err = i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			connector, err := i.KafkaClient.GetConnector(
				i.KafkaConnectors.DebeziumConnectorName(pipeline.Status.PipelineVersion), i.KafkaClient.ConnectNamespace)
			Expect(err).ToNot(HaveOccurred())
			spec := connector.Object["spec"].(map[string]interface{})
			config := spec["config"].(map[string]interface{})
			Expect(config["errors.log.enable"]).To(Equal(false))
		})

		It("Triggers refresh if configmap changes", func() {
			cm := map[string]string{
				"debezium.connector.errors.log.enable": "true",
			}
			err := i.CreateConfigMap("xjoin", cm)
			Expect(err).ToNot(HaveOccurred())
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			activeIndex := pipeline.Status.ActiveIndexName

			ctx, cancel := utils.DefaultContext()
			defer cancel()
			configMap, err := k8sUtils.FetchConfigMap(Client, i.NamespacedName.Namespace, "xjoin", ctx)
			Expect(err).ToNot(HaveOccurred())

			cm["debezium.connector.errors.log.enable"] = "false"

			configMap.Data = cm
			err = Client.Update(ctx, configMap)
			Expect(err).ToNot(HaveOccurred())

			pipeline, err = i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			connector, err := i.KafkaClient.GetConnector(
				i.KafkaConnectors.DebeziumConnectorName(pipeline.Status.PipelineVersion), i.KafkaClient.ConnectNamespace)
			Expect(err).ToNot(HaveOccurred())
			spec := connector.Object["spec"].(map[string]interface{})
			config := spec["config"].(map[string]interface{})
			Expect(config["errors.log.enable"]).To(Equal(false))
		})

		It("Triggers refresh if database secret changes", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			activeIndex := pipeline.Status.ActiveIndexName

			ctx, cancel := utils.DefaultContext()
			defer cancel()
			secret, err := k8sUtils.FetchSecret(Client, i.NamespacedName.Namespace, i.Parameters.HBIDBSecretName.String(), ctx)
			Expect(err).ToNot(HaveOccurred())

			//update the secret with new username/password
			tempUser := "tempuser"
			tempPassword := "temppassword"
			_, _ = i.DbClient.ExecQuery( //allow this to fail when the user already exists
				fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s' IN ROLE insights;", tempUser, tempPassword))
			secret.Data["db.user"] = []byte(tempUser)
			secret.Data["db.password"] = []byte(tempPassword)
			err = Client.Update(ctx, secret)
			Expect(err).ToNot(HaveOccurred())

			//run a reconcile
			pipeline, err = i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			//the new pipeline should use the updated username/password from the HBI DB secret
			connector, err := i.KafkaClient.GetConnector(
				i.KafkaConnectors.DebeziumConnectorName(pipeline.Status.PipelineVersion), i.KafkaClient.ConnectNamespace)
			Expect(err).ToNot(HaveOccurred())
			connectorSpec := connector.Object["spec"].(map[string]interface{})
			connectorConfig := connectorSpec["config"].(map[string]interface{})
			Expect(connectorConfig["database.user"]).To(Equal(tempUser))
			Expect(connectorConfig["database.password"]).To(Equal(tempPassword))
		})

		It("Triggers refresh if elasticsearch secret changes", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			activeIndex := pipeline.Status.ActiveIndexName

			ctx, cancel := utils.DefaultContext()
			defer cancel()
			secret, err := k8sUtils.FetchSecret(Client, i.NamespacedName.Namespace, i.Parameters.ElasticSearchSecretName.String(), ctx)
			Expect(err).ToNot(HaveOccurred())

			//change the secret hash by adding a new field
			secret.Data["newfield"] = []byte("value")
			err = Client.Update(ctx, secret)
			Expect(err).ToNot(HaveOccurred())

			pipeline, err = i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			connector, err := i.KafkaClient.GetConnector(
				i.KafkaConnectors.ESConnectorName(pipeline.Status.PipelineVersion), i.KafkaClient.ConnectNamespace)
			Expect(err).ToNot(HaveOccurred())
			connectorSpec := connector.Object["spec"].(map[string]interface{})
			connectorConfig := connectorSpec["config"].(map[string]interface{})
			Expect(connectorConfig["connection.username"]).To(Equal("test"))
			Expect(connectorConfig["connection.password"]).To(Equal("test1337"))
			Expect(connectorConfig["connection.url"]).To(Equal("http://xjoin-elasticsearch-es-default.test.svc:9200"))
		})

		It("Triggers refresh if index disappears", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			err = i.EsClient.DeleteIndex(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			pipeline, err = i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).To(Equal("")) //index was removed so there's no active index
			pipeline, err = i.ExpectValidReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).ToNot(Equal(""))
		})

		It("Triggers refresh if elasticsearch connector disappears", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			activeIndex := pipeline.Status.ActiveIndexName
			err = i.KafkaClient.DeleteConnector(i.KafkaConnectors.ESConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			pipeline, err = i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))
			pipeline, err = i.ExpectValidReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).ToNot(Equal(activeIndex))
		})

		It("Triggers refresh if database connector disappears", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			activeIndex := pipeline.Status.ActiveIndexName
			err = i.KafkaClient.DeleteConnector(i.KafkaConnectors.DebeziumConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			pipeline, err = i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))
			pipeline, err = i.ExpectValidReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).ToNot(Equal(activeIndex))
		})

		It("Triggers refresh if ES pipeline disappears", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			activeIndex := pipeline.Status.ActiveIndexName
			err = i.EsClient.DeleteESPipelineByVersion(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			pipeline, err = i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))
			pipeline, err = i.ExpectValidReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).ToNot(Equal(activeIndex))
		})
	})

	Describe("Spec changed", func() {
		It("Triggers refresh if resource name prefix changes", func() {
			err := i.TestSpecFieldChanged("ResourceNamePrefix", "prefixupdated", reflect.String)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Triggers refresh if KafkaCluster changes", func() {
			err := i.TestSpecFieldChanged("KafkaCluster", "newCluster", reflect.String)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Triggers refresh if KafkaClusterNamespace changes", func() {
			err := i.TestSpecFieldChanged("KafkaClusterNamespace", i.NamespacedName.Namespace, reflect.String)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Triggers refresh if ConnectCluster changes", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())

			defer gock.Off()

			gock.New("http://newCluster-connect-api.test.svc:8083").
				Get("/connectors/"+pipeline.Status.ActiveESConnectorName+"/status").
				Reply(200).
				BodyString("{}").
				AddHeader("Content-Type", "application/json")

			gock.New("http://newCluster-connect-api.test.svc:8083").
				Get("/connectors/"+pipeline.Status.ActiveDebeziumConnectorName+"/status").
				Reply(200).
				BodyString("{}").
				AddHeader("Content-Type", "application/json")

			err = i.TestSpecFieldChangedForPipeline(
				pipeline, "ConnectCluster", "newCluster", reflect.String)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Triggers refresh if ConnectClusterNamespace changes", func() {
			err := i.TestSpecFieldChanged("ConnectClusterNamespace", i.NamespacedName.Namespace, reflect.String)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Triggers refresh if HBIDBSecretName changes", func() {
			newSecretName := "host-inventory-db-new"
			err := i.CopySecret("host-inventory-db", newSecretName, i.NamespacedName.Namespace, i.NamespacedName.Namespace)
			Expect(err).ToNot(HaveOccurred())
			err = i.TestSpecFieldChanged("HBIDBSecretName", newSecretName, reflect.String)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Triggers refresh if ElasticSearchSecretName changes", func() {
			err := i.TestSpecFieldChanged("ElasticSearchSecretName", "newSecret", reflect.String)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("-> Removed", func() {
		It("Artifacts removed when initializing pipeline is removed", func() {
			err := i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			pipeline, err := i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			err = i.DeletePipeline(pipeline)
			Expect(err).ToNot(HaveOccurred())
			err = i.ExpectPipelineVersionToBeRemoved(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Artifacts removed when new pipeline is removed", func() {
			err := i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			pipeline, err := i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			err = i.DeletePipeline(pipeline)
			Expect(err).ToNot(HaveOccurred())
			err = i.ExpectPipelineVersionToBeRemoved(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Artifacts removed when valid pipeline is removed", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			err = i.DeletePipeline(pipeline)
			Expect(err).ToNot(HaveOccurred())
			err = i.ExpectPipelineVersionToBeRemoved(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Artifacts removed when refreshing pipeline is removed", func() {
			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			activeIndex := pipeline.Status.ActiveIndexName
			firstVersion := pipeline.Status.PipelineVersion

			//trigger refresh so there is an active and initializing pipeline
			ctx, cancel := utils.DefaultContext()
			defer cancel()
			secret, err := k8sUtils.FetchSecret(Client, i.NamespacedName.Namespace, i.Parameters.ElasticSearchSecretName.String(), ctx)
			Expect(err).ToNot(HaveOccurred())
			secret.Data["newfield"] = []byte("value")
			err = Client.Update(ctx, secret)
			Expect(err).ToNot(HaveOccurred())
			pipeline, err = i.ExpectInitSyncUnknownReconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))
			secondVersion := pipeline.Status.PipelineVersion

			//give connect time to create resources
			time.Sleep(10 * time.Second)

			err = i.DeletePipeline(pipeline)
			Expect(err).ToNot(HaveOccurred())
			err = i.ExpectPipelineVersionToBeRemoved(firstVersion)
			Expect(err).ToNot(HaveOccurred())
			err = i.ExpectPipelineVersionToBeRemoved(secondVersion)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Artifacts removed when an error occurs during initial setup", func() {
			ctx, cancel := utils.DefaultContext()
			defer cancel()
			secret, err := k8sUtils.FetchSecret(Client, i.NamespacedName.Namespace, i.Parameters.HBIDBSecretName.String(), ctx)
			Expect(err).ToNot(HaveOccurred())
			secret.Data["db.host"] = []byte("invalidurl")
			err = Client.Update(ctx, secret)
			Expect(err).ToNot(HaveOccurred())

			//this will fail due to incorrect secret
			err = i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			err = i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())

			exists, err := i.EsClient.IndexExists(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(Equal(false))

			slots, err := i.DbClient.ListReplicationSlotsForPrefix(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(slots)).To(Equal(0))

			versions, err := i.KafkaTopics.ListTopicNamesForPrefix(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(versions)).To(Equal(0))

			connectors, err := i.KafkaClient.ListConnectorNamesForPrefix(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(connectors)).To(Equal(0))
		})
	})

	Describe("Failures", func() {
		It("Fails if ElasticSearch secret is misconfigured", func() {
			ctx, cancel := utils.DefaultContext()
			defer cancel()
			secret, err := k8sUtils.FetchSecret(Client, i.NamespacedName.Namespace, i.Parameters.ElasticSearchSecretName.String(), ctx)
			Expect(err).ToNot(HaveOccurred())
			secret.Data["endpoint"] = []byte("invalidurl")
			err = Client.Update(ctx, secret)
			Expect(err).ToNot(HaveOccurred())

			err = i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			err = i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HavePrefix(`unsupported protocol scheme`))
		})

		It("Fails if HBI DB secret is misconfigured", func() {
			ctx, cancel := utils.DefaultContext()
			defer cancel()
			secret, err := k8sUtils.FetchSecret(Client, i.NamespacedName.Namespace, i.Parameters.HBIDBSecretName.String(), ctx)
			Expect(err).ToNot(HaveOccurred())
			secret.Data["db.host"] = []byte("invalidurl")
			err = Client.Update(ctx, secret)
			Expect(err).ToNot(HaveOccurred())

			err = i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			err = i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HavePrefix(`error connecting to invalidurl`))
		})

		It("Fails if the unable to create Kafka Topic", func() {
			namespace := "invalid"
			spec := xjoin.XJoinPipelineSpec{
				KafkaClusterNamespace: &namespace,
			}
			err := i.CreatePipeline(&spec)
			Expect(err).ToNot(HaveOccurred())
			err = i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HavePrefix(`namespaces "invalid" not found`))

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(1))
		})

		It("Fails if the unable to create Elasticsearch Connector", func() {
			cm := map[string]string{
				"elasticsearch.connector.config": "invalid",
			}
			err := i.CreateConfigMap("xjoin", cm)
			Expect(err).ToNot(HaveOccurred())
			err = i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			err = i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HavePrefix(`invalid character 'i' looking for beginning of value`))

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(1))
		})

		It("Fails if the unable to create Debezium Connector", func() {
			cm := map[string]string{
				"debezium.connector.config": "invalid",
			}
			err := i.CreateConfigMap("xjoin", cm)
			Expect(err).ToNot(HaveOccurred())
			err = i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			err = i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HavePrefix(`invalid character 'i' looking for beginning of value`))

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(1))
		})

		It("Fails if ES Index cannot be created", func() {
			invalidPrefix := "invalidPrefix"
			spec := &xjoin.XJoinPipelineSpec{
				ResourceNamePrefix: &invalidPrefix,
			}
			err := i.CreatePipeline(spec)
			Expect(err).ToNot(HaveOccurred())
			err = i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(1))
		})

		It("Fails if ESPipeline cannot be created", func() {
			cm := map[string]string{
				"elasticsearch.pipeline.template": "invalid",
			}
			err := i.CreateConfigMap("xjoin", cm)
			Expect(err).ToNot(HaveOccurred())
			err = i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			err = i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(1))
		})
	})

	Describe("Deviation", func() {
		It("Restarts failed ES connector task without a refresh", func() {
			serviceName := "xjoin-elasticsearch-es-default-new"
			defer func(i *Iteration, serviceName string) {
				err := i.DeleteService(serviceName)
				Expect(err).ToNot(HaveOccurred())
			}(i, serviceName)

			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			activePipelineVersion := pipeline.Status.ActivePipelineVersion

			err = i.WaitForConnectorToBeCreated(pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())

			//pause connector reconciliation, so it can be modified to an invalid state
			defer func(i *Iteration, connectorName string) {
				err := i.ResumeConnectorReconciliation(connectorName)
				Expect(err).ToNot(HaveOccurred())
			}(i, pipeline.Status.ActiveESConnectorName)
			err = i.PauseConnectorReconciliation(pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())

			//update es connector config with invalid url
			err = i.SetESConnectorURL(
				"http://"+serviceName+".test.svc:9200",
				pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())

			err = i.WaitForConnectorTaskToFail(pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())

			//create service with invalid url
			err = i.CreateESService(serviceName)
			Expect(err).ToNot(HaveOccurred())

			//give the service time to be fully created
			time.Sleep(5 * time.Second)

			//reconcile
			pipeline, err = i.ExpectValidReconcile()
			Expect(err).ToNot(HaveOccurred())

			//validate task is running
			newTasks, err := i.KafkaConnectors.ListConnectorTasks(pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())
			for _, task := range newTasks {
				Expect(task["state"]).To(Equal("RUNNING"))
			}

			//validate no refresh occurred
			Expect(pipeline.Status.ActivePipelineVersion).To(Equal(activePipelineVersion))
		})

		It("Restarts failed DB connector task without a refresh", func() {
			dbHost := "host-inventory-db-new"
			defer func(i *Iteration, serviceName string) {
				err := i.DeleteService(serviceName)
				Expect(err).ToNot(HaveOccurred())
			}(i, dbHost)
			defer ForwardPorts()

			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())
			activePipelineVersion := pipeline.Status.ActivePipelineVersion

			err = i.WaitForConnectorToBeCreated(pipeline.Status.ActiveDebeziumConnectorName)
			Expect(err).ToNot(HaveOccurred())

			//pause connector reconciliation, so it can be modified to an invalid state
			defer func(i *Iteration, connectorName string) {
				err := i.ResumeConnectorReconciliation(connectorName)
				Expect(err).ToNot(HaveOccurred())
			}(i, pipeline.Status.ActiveDebeziumConnectorName)
			err = i.PauseConnectorReconciliation(pipeline.Status.ActiveDebeziumConnectorName)
			Expect(err).ToNot(HaveOccurred())

			//create the service, do the PUT, then delete the service
			//otherwise the PUT to connector/config will error with 500
			err = i.CreateDBService(dbHost)
			Expect(err).ToNot(HaveOccurred())
			time.Sleep(2 * time.Second)
			err = i.SetDBConnectorHost(
				dbHost+".test.svc",
				pipeline.Status.ActiveDebeziumConnectorName)
			Expect(err).ToNot(HaveOccurred())
			time.Sleep(2 * time.Second)
			err = i.DeleteService(dbHost)
			Expect(err).ToNot(HaveOccurred())

			//give the connector time to realize it can't connect
			time.Sleep(10 * time.Second)
			err = i.KafkaConnectors.RestartTaskForConnector(pipeline.Status.ActiveDebeziumConnectorName, 0)
			Expect(err).ToNot(HaveOccurred())

			//validate task is failed
			err = i.WaitForConnectorTaskToFail(pipeline.Status.ActiveDebeziumConnectorName)
			Expect(err).ToNot(HaveOccurred())

			//create service with invalid url
			err = i.CreateDBService(dbHost)
			Expect(err).ToNot(HaveOccurred())

			//give the service time to be fully created
			time.Sleep(5 * time.Second)

			//reconcile
			pipeline, err = i.ExpectValidReconcile()
			Expect(err).ToNot(HaveOccurred())

			//validate task is running
			newTasks, err := i.KafkaConnectors.ListConnectorTasks(pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())
			for _, task := range newTasks {
				Expect(task["state"]).To(Equal("RUNNING"))
			}

			//validate no refresh occurred
			Expect(pipeline.Status.ActivePipelineVersion).To(Equal(activePipelineVersion))
		})

		It("Fails if unable to get connector task status", func() {
			defer gock.Off()

			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())

			gock.New("http://connect-connect-api.test.svc:8083").
				Get("/connectors/" + pipeline.Status.ActiveESConnectorName + "/status").
				Reply(500)

			err = i.ReconcileValidationWithError()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HavePrefix("unable to get connector"))
			Expect(err.Error()).To(HaveSuffix("status"))
		})

		It("Performs a refresh when unable to successfully restart failed connector task", func() {
			Skip("unreliable")
			serviceName := "xjoin-elasticsearch-es-default-new"

			cm := map[string]string{
				"init.validation.attempts.threshold": "1",
				"validation.attempts.threshold":      "1",
			}
			err := i.CreateConfigMap("xjoin", cm)
			Expect(err).ToNot(HaveOccurred())

			pipeline, err := i.CreateValidPipeline()
			Expect(err).ToNot(HaveOccurred())

			//give connect time to create the connectors
			time.Sleep(15 * time.Second)

			//pause reconciliation of ES connector
			defer func(i *Iteration, connectorName string) {
				err := i.ResumeConnectorReconciliation(connectorName)
				Expect(err).ToNot(HaveOccurred())
			}(i, pipeline.Status.ActiveESConnectorName)
			err = i.PauseConnectorReconciliation(pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())

			//update es connector config with invalid url
			err = i.SetESConnectorURL(
				"http://"+serviceName+":9200",
				pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())

			//give the connector time to realize it can't connect
			time.Sleep(10 * time.Second)

			//validate task is failed
			tasks, err := i.KafkaConnectors.ListConnectorTasks(pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(tasks)).To(BeNumerically(">", 0))
			for _, task := range tasks {
				Expect(task["state"]).To(Equal("FAILED"))
			}

			//reconcile
			pipeline, err = i.ReconcileValidation()
			Expect(err).ToNot(HaveOccurred())
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_NEW))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))
		})
	})

	Describe("Kafka Topic", func() {
		It("Waits for the Kafka topic to be ready before creating connectors", func() {
			cm := map[string]string{
				"kafka.topic.replicas": "1",
			}
			err := i.CreateConfigMap("xjoin", cm)
			Expect(err).ToNot(HaveOccurred())

			err = i.CreatePipeline()
			Expect(err).ToNot(HaveOccurred())
			pipeline, err := i.ReconcileXJoinNonTest()
			Expect(err).ToNot(HaveOccurred())
			topic, err := i.KafkaTopics.GetTopic(i.KafkaTopics.TopicName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			topicStruct := topic.(*unstructured.Unstructured)
			status := topicStruct.Object["status"].(map[string]interface{})
			conditions := status["conditions"].([]interface{})
			readyCondition := conditions[0].(map[string]interface{})
			Expect(readyCondition["status"]).To(Equal("True"))
		})

		It("Fails if topic is not ready before timeout", func() {
			cm := map[string]string{
				"kafka.topic.creation.timeout": "1",
			}
			err := i.CreateConfigMap("xjoin", cm)
			Expect(err).ToNot(HaveOccurred())
			clusterName := "invalid.cluster"
			pipeline := xjoin.XJoinPipelineSpec{
				KafkaCluster: &clusterName,
			}
			err = i.CreatePipeline(&pipeline)
			Expect(err).ToNot(HaveOccurred())
			err = i.ReconcileXJoinNonTestWithError()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HavePrefix(`timed out waiting for Kafka Topic`))
		})

		It("Testing the failures", func() {
			for j := 0; j < 1; j++ {
				log.Info("ITERATION: " + strconv.Itoa(j))

				version := strconv.FormatInt(time.Now().UnixNano(), 10)

				log.Info("CREATING TOPIC " + version)
				_, err := i.KafkaTopics.CreateTopic(version, false)
				if err != nil {
					return
				}

				//log.Info("CREATING DB CONNECTOR" + version)
				//_, err := i.KafkaClient.CreateDebeziumConnector(version, false)
				//Expect(err).ToNot(HaveOccurred())

				log.Info("CREATING ES CONNECTOR" + version)
				_, err = i.KafkaConnectors.CreateESConnector(version, false)
				Expect(err).ToNot(HaveOccurred())

				//log.Info("DELETING DB CONNECTOR" + version)
				//err = i.KafkaClient.DeleteConnector("xjointest.db." + version)
				//Expect(err).ToNot(HaveOccurred())

				log.Info("DELETING ES CONNECTOR" + version)
				err = i.KafkaClient.DeleteConnector("xjointest.es." + version)
				Expect(err).ToNot(HaveOccurred())

				log.Info("DELETING TOPIC" + version)
				err = i.KafkaTopics.DeleteTopicByPipelineVersion(version)
				Expect(err).ToNot(HaveOccurred())

				time.Sleep(100 * time.Millisecond)

			}
		})
	})
})
