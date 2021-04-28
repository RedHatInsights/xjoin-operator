package test

import (
	"context"
	"fmt"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"github.com/redhatinsights/xjoin-operator/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestControllers(t *testing.T) {
	test.Setup(t, "Controllers")
}

var _ = Describe("Pipeline operations", func() {
	var i *Iteration

	BeforeEach(func() {
		i = Before()
	})

	AfterEach(func() {
		After(i)
	})

	Describe("New -> InitialSync", func() {
		It("Creates a connector, ES Index, and topic for a new pipeline", func() {
			i.CreatePipeline()
			i.ReconcileXJoin()

			pipeline := i.GetPipeline()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			dbConnector, err := i.KafkaClient.GetConnector(
				i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			Expect(dbConnector.GetLabels()["strimzi.io/cluster"]).To(Equal("xjoin-kafka-connect-strimzi"))
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
			Expect(dbConnectorConfig["database.hostname"]).To(Equal("inventory-db"))
			Expect(dbConnectorConfig["errors.log.include.messages"]).To(Equal(true))
			Expect(dbConnectorConfig["max.queue.size"]).To(Equal(int64(1000)))
			Expect(dbConnectorConfig["poll.interval.ms"]).To(Equal(int64(100)))
			Expect(dbConnectorConfig["slot.name"]).To(Equal(ResourceNamePrefix + "_" + pipeline.Status.PipelineVersion))
			Expect(dbConnectorConfig["table.whitelist"]).To(Equal("public.hosts"))
			Expect(dbConnectorConfig["database.server.name"]).To(Equal(ResourceNamePrefix + "." + pipeline.Status.PipelineVersion))
			Expect(dbConnectorConfig["errors.log.enable"]).To(Equal(true))
			Expect(dbConnectorConfig["transforms.unwrap.type"]).To(Equal("io.debezium.transforms.ExtractNewRecordState"))

			esConnector, err := i.KafkaClient.GetConnector(
				i.KafkaClient.ESConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			Expect(esConnector.GetLabels()["strimzi.io/cluster"]).To(Equal("xjoin-kafka-connect-strimzi"))
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
			Expect(esConnectorConfig["connection.url"]).To(Equal("http://xjoin-elasticsearch-es-http:9200"))
			Expect(esConnectorConfig["errors.log.enable"]).To(Equal(true))
			Expect(esConnectorConfig["max.retries"]).To(Equal(int64(8)))
			Expect(esConnectorConfig["transforms.deleteIf.field"]).To(Equal("__deleted"))
			Expect(esConnectorConfig["transforms.extractKey.field"]).To(Equal("id"))
			Expect(esConnectorConfig["transforms.flattenListString.type"]).To(Equal("com.redhat.insights.flattenlistsmt.FlattenList$Value"))
			Expect(esConnectorConfig["transforms.valueToKey.type"]).To(Equal("org.apache.kafka.connect.transforms.ValueToKey"))
			Expect(esConnectorConfig["transforms"]).To(Equal("valueToKey, extractKey, expandJSON, deleteIf, flattenList, flattenListString, renameTopic"))
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
			Expect(esConnectorConfig["connection.password"]).To(Equal("test1337"))
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

			topics, err := i.KafkaClient.ListTopicNamesForPipelineVersion(pipeline.Status.PipelineVersion)
			Expect(topics).To(ContainElement(i.KafkaClient.TopicName(pipeline.Status.PipelineVersion)))
		})

		It("Creates ESPipeline for new xjoin pipeline", func() {
			i.CreatePipeline()
			i.ReconcileXJoin()

			pipeline := i.GetPipeline()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			esPipeline, err := i.EsClient.GetESPipeline(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			Expect(esPipeline).ToNot(BeEmpty())
			Expect(esPipeline).To(HaveKey(i.EsClient.ESPipelineName(pipeline.Status.PipelineVersion)))
		})

		It("Considers configmap configuration", func() {
			i.CreatePipeline()

			cm := map[string]string{
				"connect.cluster":                                     "test.connect.cluster",
				"kafka.cluster":                                       "test.kafka.cluster",
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

			i.CreateConfigMap("xjoin", cm)
			i.ReconcileXJoin()

			pipeline := i.GetPipeline()
			esConnector, err := i.KafkaClient.GetConnector(
				i.KafkaClient.ESConnectorName(pipeline.Status.PipelineVersion))

			Expect(err).ToNot(HaveOccurred())
			Expect(esConnector.GetLabels()["strimzi.io/cluster"]).To(Equal(cm["connect.cluster"]))
			Expect(esConnector.GetName()).To(Equal(ResourceNamePrefix + ".es." + pipeline.Status.PipelineVersion))
			esConnectorSpec := esConnector.Object["spec"].(map[string]interface{})
			esConnectorConfig := esConnectorSpec["config"].(map[string]interface{})
			Expect(esConnectorConfig["tasks.max"]).To(Equal(cm["elasticsearch.connector.tasks.max"]))
			Expect(esConnectorConfig["topics"]).To(Equal(ResourceNamePrefix + "." + pipeline.Status.PipelineVersion + ".public.hosts"))
			Expect(esConnectorConfig["batch.size"]).To(Equal(test.StrToInt64(cm["elasticsearch.connector.batch.size"])))
			Expect(esConnectorConfig["max.in.flight.requests"]).To(Equal(test.StrToInt64(cm["elasticsearch.connector.max.in.flight.requests"])))
			Expect(esConnectorConfig["errors.log.enable"]).To(Equal(test.StrToBool(cm["elasticsearch.connector.errors.log.enable"])))
			Expect(esConnectorConfig["max.retries"]).To(Equal(test.StrToInt64(cm["elasticsearch.connector.max.retries"])))
			Expect(esConnectorConfig["retry.backoff.ms"]).To(Equal(test.StrToInt64(cm["elasticsearch.connector.retry.backoff.ms"])))
			Expect(esConnectorConfig["max.buffered.records"]).To(Equal(test.StrToInt64(cm["elasticsearch.connector.max.buffered.records"])))
			Expect(esConnectorConfig["linger.ms"]).To(Equal(test.StrToInt64(cm["elasticsearch.connector.linger.ms"])))

			dbConnector, err := i.KafkaClient.GetConnector(
				i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion))

			Expect(err).ToNot(HaveOccurred())
			Expect(dbConnector.GetLabels()["strimzi.io/cluster"]).To(Equal(cm["connect.cluster"]))
			Expect(dbConnector.GetName()).To(Equal(ResourceNamePrefix + ".db." + pipeline.Status.PipelineVersion))
			dbConnectorSpec := dbConnector.Object["spec"].(map[string]interface{})
			dbConnectorConfig := dbConnectorSpec["config"].(map[string]interface{})
			Expect(dbConnectorConfig["tasks.max"]).To(Equal(cm["debezium.connector.tasks.max"]))
			Expect(dbConnectorConfig["max.batch.size"]).To(Equal(test.StrToInt64(cm["debezium.connector.max.batch.size"])))
			Expect(dbConnectorConfig["max.queue.size"]).To(Equal(test.StrToInt64(cm["debezium.connector.max.queue.size"])))
			Expect(dbConnectorConfig["poll.interval.ms"]).To(Equal(test.StrToInt64(cm["debezium.connector.poll.interval.ms"])))
			Expect(dbConnectorConfig["errors.log.enable"]).To(Equal(test.StrToBool(cm["debezium.connector.errors.log.enable"])))
		})

		It("Considers db secret name configuration", func() {
			hbiDBSecret, err := utils.FetchSecret(
				test.Client, i.NamespacedName.Namespace, i.Parameters.HBIDBSecretName.String())
			Expect(err).ToNot(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()
			err = test.Client.Delete(ctx, hbiDBSecret)
			Expect(err).ToNot(HaveOccurred())

			secretName := "test-hbi-db-secret"
			i.CreateDbSecret(secretName)

			i.CreatePipeline(&xjoin.XJoinPipelineSpec{HBIDBSecretName: &secretName})
			i.ReconcileXJoin() //this will fail if the secret is missing
		})

		It("Considers es secret name configuration", func() {
			elasticSearchSecret, err := utils.FetchSecret(test.Client, i.NamespacedName.Namespace, i.Parameters.ElasticSearchSecretName.String())
			Expect(err).ToNot(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()
			err = test.Client.Delete(ctx, elasticSearchSecret)
			Expect(err).ToNot(HaveOccurred())

			secretName := "test-elasticsearch-secret"
			i.CreateESSecret(secretName)

			i.CreatePipeline(&xjoin.XJoinPipelineSpec{ElasticSearchSecretName: &secretName})
			i.ReconcileXJoin() //this will fail if the secret is missing
		})

		It("Removes stale connectors", func() {
			_, err := i.KafkaClient.CreateDebeziumConnector("1", false)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.KafkaClient.CreateDebeziumConnector("2", false)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.KafkaClient.CreateESConnector("1", false)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.KafkaClient.CreateESConnector("2", false)
			Expect(err).ToNot(HaveOccurred())

			connectors, err := i.KafkaClient.ListConnectors()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(connectors.Items)).To(Equal(4))

			i.CreatePipeline()
			i.ReconcileXJoin()
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

			i.CreatePipeline()
			i.ReconcileXJoin()
			indices, err = i.EsClient.ListIndices()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(indices)).To(Equal(1))
		})

		It("Removes stale topics", func() {
			_, err := i.KafkaClient.CreateTopic("1", false)
			Expect(err).ToNot(HaveOccurred())
			_, err = i.KafkaClient.CreateTopic("2", false)
			Expect(err).ToNot(HaveOccurred())

			topics, err := i.KafkaClient.ListTopicNamesForPrefix(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(topics)).To(Equal(2))

			i.CreatePipeline()
			i.ReconcileXJoin()

			topics, err = i.KafkaClient.ListTopicNamesForPrefix(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(topics)).To(Equal(1))
		})

		It("Removes stale replication slots", func() {
			prefix := ResourceNamePrefix + ".withadot"
			slot1 := database.ReplicationSlotPrefix(prefix) + "_1"
			slot2 := database.ReplicationSlotPrefix(prefix) + "_2"
			err := i.DbClient.RemoveReplicationSlotsForPrefix(prefix)
			Expect(err).ToNot(HaveOccurred())
			err = i.DbClient.CreateReplicationSlot(slot1)
			Expect(err).ToNot(HaveOccurred())
			err = i.DbClient.CreateReplicationSlot(slot2)
			Expect(err).ToNot(HaveOccurred())

			slots, err := i.DbClient.ListReplicationSlots(prefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(slots).To(ContainElements(slot1, slot2))

			i.CreatePipeline(&xjoin.XJoinPipelineSpec{ResourceNamePrefix: &prefix})
			i.ReconcileXJoin()

			slots, err = i.DbClient.ListReplicationSlots(prefix)
			Expect(err).ToNot(HaveOccurred())

			Expect(slots).ToNot(ContainElements(slot1, slot2))
		})
	})

	Describe("InitialSync -> Valid", func() {
		It("Creates the elasticsearch alias", func() {
			i.CreatePipeline()
			i.ReconcileXJoin()
			pipeline := i.GetPipeline()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			i.ReconcileValidation()
			pipeline = i.ReconcileXJoin()

			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))
		})

		It("Triggers refresh if pipeline fails to become valid for too long", func() {
			i.CreatePipeline()

			cm := map[string]string{
				"init.validation.attempts.threshold": "2",
			}

			i.CreateConfigMap("xjoin", cm)
			i.ReconcileXJoin()
			pipeline := i.GetPipeline()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			err := i.KafkaClient.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			hostId := i.InsertHost()

			i.ReconcileValidation()
			pipeline = i.ReconcileXJoin()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))

			i.ReconcileValidation()
			pipeline = i.ReconcileXJoin()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_NEW))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))
			Expect(pipeline.Status.PipelineVersion).To(Equal(""))

			i.ReconcileValidation()
			pipeline = i.ReconcileXJoin()
			Expect(pipeline.Status.PipelineVersion).ToNot(Equal(""))
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INITIAL_SYNC))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeTrue())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))

			i.IndexDocument(pipeline.Status.PipelineVersion, hostId, "es.document.1")

			i.ReconcileValidation()
			pipeline = i.ReconcileXJoin()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))
		})

		It("Sets active resource names for a valid pipeline", func() {
			pipeline := i.CreateValidPipeline()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(i.EsClient.ESIndexName(pipeline.Status.PipelineVersion)))
			Expect(pipeline.Status.ActiveTopicName).To(Equal(i.KafkaClient.TopicName(pipeline.Status.PipelineVersion)))
			Expect(pipeline.Status.ActiveAliasName).To(Equal(i.EsClient.AliasName()))
			Expect(pipeline.Status.ActiveDebeziumConnectorName).To(
				Equal(i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion)))
			Expect(pipeline.Status.ActiveESConnectorName).To(
				Equal(i.KafkaClient.ESConnectorName(pipeline.Status.PipelineVersion)))
		})
	})

	Describe("Invalid -> New", func() {
		Context("In a refresh", func() {
			It("Keeps the old table active until the new one is valid", func() {
				cm := map[string]string{
					"validation.attempts.threshold": "2",
				}
				i.CreateConfigMap("xjoin", cm)

				pipeline := i.CreateValidPipeline()
				activeIndex := pipeline.Status.ActiveIndexName

				err := i.KafkaClient.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
				Expect(err).ToNot(HaveOccurred())
				hostId := i.InsertHost()

				pipeline = i.ExpectInvalidReconcile()
				Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

				pipeline = i.ExpectNewReconcile()
				Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

				pipeline = i.ExpectInitSyncUnknownReconcile()
				Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

				i.IndexDocument(pipeline.Status.PipelineVersion, hostId, "es.document.1")

				pipeline = i.ExpectValidReconcile()
				Expect(pipeline.Status.ActiveIndexName).ToNot(Equal(activeIndex))
			})
		})
	})

	Describe("Valid -> New", func() {
		It("Preserves active resource names during refresh", func() {
			pipeline := i.CreateValidPipeline()
			activeIndexName := i.EsClient.ESIndexName(pipeline.Status.PipelineVersion)
			activeTopicName := i.KafkaClient.TopicName(pipeline.Status.PipelineVersion)
			activeAliasName := i.EsClient.AliasName()
			activeDebeziumConnectorName := i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion)
			activeESConnectorName := i.KafkaClient.ESConnectorName(pipeline.Status.PipelineVersion)

			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndexName))
			Expect(pipeline.Status.ActiveTopicName).To(Equal(activeTopicName))
			Expect(pipeline.Status.ActiveAliasName).To(Equal(activeAliasName))
			Expect(pipeline.Status.ActiveDebeziumConnectorName).To(Equal(activeDebeziumConnectorName))
			Expect(pipeline.Status.ActiveESConnectorName).To(Equal(activeESConnectorName))

			//trigger refresh with a new configmap
			clusterName := "invalid.cluster"
			cm := map[string]string{
				"connect.cluster": clusterName,
			}
			i.CreateConfigMap("xjoin", cm)

			pipeline = i.ExpectInitSyncUnknownReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndexName))
			Expect(pipeline.Status.ActiveTopicName).To(Equal(activeTopicName))
			Expect(pipeline.Status.ActiveAliasName).To(Equal(activeAliasName))
			Expect(pipeline.Status.ActiveDebeziumConnectorName).To(Equal(activeDebeziumConnectorName))
			Expect(pipeline.Status.ActiveESConnectorName).To(Equal(activeESConnectorName))
		})

		It("Triggers refresh if configmap is created", func() {
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName

			clusterName := "invalid.cluster"
			cm := map[string]string{
				"connect.cluster": clusterName,
			}
			i.CreateConfigMap("xjoin", cm)

			pipeline = i.ExpectInitSyncUnknownReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			connector, err := i.KafkaClient.GetConnector(i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			Expect(connector.GetLabels()["strimzi.io/cluster"]).To(Equal(clusterName))
		})

		It("Triggers refresh if configmap changes", func() {
			cm := map[string]string{
				"connect.cluster": i.Parameters.ConnectCluster.String(),
			}
			i.CreateConfigMap("xjoin", cm)
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName

			configMap, err := utils.FetchConfigMap(test.Client, i.NamespacedName.Namespace, "xjoin")
			Expect(err).ToNot(HaveOccurred())
			updatedClusterName := "invalid.cluster"
			cm["connect.cluster"] = updatedClusterName
			configMap.Data = cm
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()
			err = test.Client.Update(ctx, configMap)
			Expect(err).ToNot(HaveOccurred())

			pipeline = i.ExpectInitSyncUnknownReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			connector, err := i.KafkaClient.GetConnector(i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			Expect(connector.GetLabels()["strimzi.io/cluster"]).To(Equal(updatedClusterName))
		})

		It("Triggers refresh if database secret changes", func() {
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName

			secret, err := utils.FetchSecret(test.Client, i.NamespacedName.Namespace, i.Parameters.HBIDBSecretName.String())
			Expect(err).ToNot(HaveOccurred())

			//update the secret with new username/password
			tempUser := "tempuser"
			tempPassword := "temppassword"
			_, _ = i.DbClient.ExecQuery( //allow this to fail when the user already exists
				fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s' IN ROLE insights;", tempUser, tempPassword))
			secret.Data["db.user"] = []byte(tempUser)
			secret.Data["db.password"] = []byte(tempPassword)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()
			err = test.Client.Update(ctx, secret)

			//run a reconcile
			pipeline = i.ExpectInitSyncUnknownReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			//the new pipeline should use the updated username/password from the HBI DB secret
			connector, err := i.KafkaClient.GetConnector(i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			connectorSpec := connector.Object["spec"].(map[string]interface{})
			connectorConfig := connectorSpec["config"].(map[string]interface{})
			Expect(connectorConfig["database.user"]).To(Equal(tempUser))
			Expect(connectorConfig["database.password"]).To(Equal(tempPassword))
		})

		It("Triggers refresh if elasticsearch secret changes", func() {
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName

			secret, err := utils.FetchSecret(test.Client, i.NamespacedName.Namespace, i.Parameters.ElasticSearchSecretName.String())
			Expect(err).ToNot(HaveOccurred())

			//change the secret hash by adding a new field
			secret.Data["newfield"] = []byte("value")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()
			err = test.Client.Update(ctx, secret)

			pipeline = i.ExpectInitSyncUnknownReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))

			connector, err := i.KafkaClient.GetConnector(i.KafkaClient.ESConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			connectorSpec := connector.Object["spec"].(map[string]interface{})
			connectorConfig := connectorSpec["config"].(map[string]interface{})
			Expect(connectorConfig["connection.username"]).To(Equal("test"))
			Expect(connectorConfig["connection.password"]).To(Equal("test1337"))
			Expect(connectorConfig["connection.url"]).To(Equal("http://xjoin-elasticsearch-es-http:9200"))
		})

		It("Triggers refresh if index disappears", func() {
			pipeline := i.CreateValidPipeline()
			err := i.EsClient.DeleteIndex(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			pipeline = i.ExpectInitSyncUnknownReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal("")) //index was removed so there's no active index
			pipeline = i.ExpectValidReconcile()
			Expect(pipeline.Status.ActiveIndexName).ToNot(Equal(""))
		})

		It("Triggers refresh if elasticsearch connector disappears", func() {
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName
			err := i.KafkaClient.DeleteConnector(i.KafkaClient.ESConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			pipeline = i.ExpectInitSyncUnknownReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))
			pipeline = i.ExpectValidReconcile()
			Expect(pipeline.Status.ActiveIndexName).ToNot(Equal(activeIndex))
		})

		It("Triggers refresh if database connector disappears", func() {
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName
			err := i.KafkaClient.DeleteConnector(i.KafkaClient.DebeziumConnectorName(pipeline.Status.PipelineVersion))
			Expect(err).ToNot(HaveOccurred())
			pipeline = i.ExpectInitSyncUnknownReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))
			pipeline = i.ExpectValidReconcile()
			Expect(pipeline.Status.ActiveIndexName).ToNot(Equal(activeIndex))
		})

		It("Triggers refresh if ES pipeline disappears", func() {
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName
			err := i.EsClient.DeleteESPipelineByVersion(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			pipeline = i.ExpectInitSyncUnknownReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))
			pipeline = i.ExpectValidReconcile()
			Expect(pipeline.Status.ActiveIndexName).ToNot(Equal(activeIndex))
		})
	})

	Describe("Spec changed", func() {
		It("Triggers refresh if resource name prefix changes", func() {
			i.TestSpecFieldChanged("ResourceNamePrefix", "xjointestupdated", reflect.String)
		})

		It("Triggers refresh if KafkaCluster changes", func() {
			i.TestSpecFieldChanged("KafkaCluster", "newCluster", reflect.String)
		})

		It("Triggers refresh if KafkaClusterNamespace changes", func() {
			i.TestSpecFieldChanged("KafkaClusterNamespace", "kafka", reflect.String)
		})

		It("Triggers refresh if ConnectCluster changes", func() {
			i.TestSpecFieldChanged("ConnectCluster", "newCluster", reflect.String)
		})

		It("Triggers refresh if ConnectClusterNamespace changes", func() {
			i.TestSpecFieldChanged("ConnectClusterNamespace", i.NamespacedName.Namespace, reflect.String)
		})

		It("Triggers refresh if HBIDBSecretName changes", func() {
			i.TestSpecFieldChanged("HBIDBSecretName", "newSecret", reflect.String)
		})

		It("Triggers refresh if ElasticSearchSecretName changes", func() {
			i.TestSpecFieldChanged("ElasticSearchSecretName", "newSecret", reflect.String)
		})
	})

	Describe("-> Removed", func() {
		It("Artifacts removed when initializing pipeline is removed", func() {
			i.CreatePipeline()
			pipeline := i.ExpectInitSyncUnknownReconcile()
			i.DeletePipeline(pipeline)
			i.ExpectPipelineVersionToBeRemoved(pipeline.Status.PipelineVersion)
		})

		It("Artifacts removed when new pipeline is removed", func() {
			i.CreatePipeline()
			pipeline := i.ExpectInitSyncUnknownReconcile()
			i.DeletePipeline(pipeline)
			i.ExpectPipelineVersionToBeRemoved(pipeline.Status.PipelineVersion)
		})

		It("Artifacts removed when valid pipeline is removed", func() {
			pipeline := i.CreateValidPipeline()
			i.DeletePipeline(pipeline)
			i.ExpectPipelineVersionToBeRemoved(pipeline.Status.PipelineVersion)
		})

		It("Artifacts removed when refreshing pipeline is removed", func() {
			pipeline := i.CreateValidPipeline()
			activeIndex := pipeline.Status.ActiveIndexName
			firstVersion := pipeline.Status.PipelineVersion

			//trigger refresh so there is an active and initializing pipeline
			secret, err := utils.FetchSecret(test.Client, i.NamespacedName.Namespace, i.Parameters.ElasticSearchSecretName.String())
			Expect(err).ToNot(HaveOccurred())
			secret.Data["newfield"] = []byte("value")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()
			err = test.Client.Update(ctx, secret)
			pipeline = i.ExpectInitSyncUnknownReconcile()
			Expect(pipeline.Status.ActiveIndexName).To(Equal(activeIndex))
			secondVersion := pipeline.Status.PipelineVersion

			//give connect time to create resources
			time.Sleep(5 * time.Second)

			i.DeletePipeline(pipeline)
			i.ExpectPipelineVersionToBeRemoved(firstVersion)
			i.ExpectPipelineVersionToBeRemoved(secondVersion)
		})

		It("Artifacts removed when an error occurs during initial setup", func() {
			secret, err := utils.FetchSecret(test.Client, i.NamespacedName.Namespace, i.Parameters.HBIDBSecretName.String())
			Expect(err).ToNot(HaveOccurred())
			secret.Data["db.host"] = []byte("invalidurl")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()
			err = test.Client.Update(ctx, secret)

			//this will fail due to incorrect secret
			i.CreatePipeline()
			err = i.ReconcileXJoinWithError()

			exists, err := i.EsClient.IndexExists(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(Equal(false))

			slots, err := i.DbClient.ListReplicationSlots(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(slots)).To(Equal(0))

			versions, err := i.KafkaClient.ListTopicNamesForPrefix(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(versions)).To(Equal(0))

			connectors, err := i.KafkaClient.ListConnectorNamesForPrefix(ResourceNamePrefix)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(connectors)).To(Equal(0))
		})
	})

	Describe("Failures", func() {
		It("Fails if ElasticSearch secret is misconfigured", func() {
			secret, err := utils.FetchSecret(test.Client, i.NamespacedName.Namespace, i.Parameters.ElasticSearchSecretName.String())
			Expect(err).ToNot(HaveOccurred())
			secret.Data["endpoint"] = []byte("invalidurl")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()
			err = test.Client.Update(ctx, secret)

			i.CreatePipeline()
			err = i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HavePrefix(`unsupported protocol scheme`))

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(3))
		})

		It("Fails if HBI DB secret is misconfigured", func() {
			secret, err := utils.FetchSecret(test.Client, i.NamespacedName.Namespace, i.Parameters.HBIDBSecretName.String())
			Expect(err).ToNot(HaveOccurred())
			secret.Data["db.host"] = []byte("invalidurl")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
			defer cancel()
			err = test.Client.Update(ctx, secret)

			i.CreatePipeline()
			err = i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(HavePrefix(`error connecting to invalidurl`))

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(1))
		})

		It("Fails if the unable to create Kafka Topic", func() {
			cm := map[string]string{
				"kafka.cluster.namespace": "invalid",
			}
			i.CreateConfigMap("xjoin", cm)
			i.CreatePipeline()
			err := i.ReconcileXJoinWithError()
			Expect(err.Error()).To(HavePrefix(`namespaces "invalid" not found`))

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(1))
		})

		It("Fails if the unable to create Elasticsearch Connector", func() {
			cm := map[string]string{
				"elasticsearch.connector.config": "invalid",
			}
			i.CreateConfigMap("xjoin", cm)
			i.CreatePipeline()
			err := i.ReconcileXJoinWithError()
			Expect(err.Error()).To(HavePrefix(`invalid character 'i' looking for beginning of value`))

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(1))
		})

		It("Fails if the unable to create Debezium Connector", func() {
			cm := map[string]string{
				"debezium.connector.config": "invalid",
			}
			i.CreateConfigMap("xjoin", cm)
			i.CreatePipeline()
			err := i.ReconcileXJoinWithError()
			Expect(err.Error()).To(HavePrefix(`invalid character 'i' looking for beginning of value`))

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(1))
		})

		It("Fails if ES Index cannot be created", func() {
			invalidPrefix := "invalidPrefix"
			spec := &xjoin.XJoinPipelineSpec{
				ResourceNamePrefix: &invalidPrefix,
			}
			i.CreatePipeline(spec)
			err := i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(1))
		})

		It("Fails if ESPipeline cannot be created", func() {
			cm := map[string]string{
				"elasticsearch.pipeline.template": "invalid",
			}
			i.CreateConfigMap("xjoin", cm)
			i.CreatePipeline()
			err := i.ReconcileXJoinWithError()
			Expect(err).To(HaveOccurred())

			recorder, _ := i.XJoinReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(1))
		})
	})

	Describe("Deviation", func() {
		It("Restarts failed ES connector task without a refresh", func() {
			serviceName := "xjoin-elasticsearch-es-default-new"
			defer i.ScaleDeployment("strimzi-cluster-operator", "kafka", 1)
			defer i.DeleteService(serviceName)

			pipeline := i.CreateValidPipeline()
			activePipelineVersion := pipeline.Status.ActivePipelineVersion

			//scale down strimzi operator deployment
			i.ScaleDeployment("strimzi-cluster-operator", "kafka", 0)

			//update es connector config with invalid url
			i.SetESConnectorURL(
				"http://"+serviceName+":9200",
				pipeline.Status.ActiveESConnectorName)

			//give the connector time to realize it can't connect
			time.Sleep(10 * time.Second)

			//validate task is failed
			tasks, err := i.KafkaClient.ListConnectorTasks(pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())
			for _, task := range tasks {
				Expect(task["state"]).To(Equal("FAILED"))
			}

			//create service with invalid url
			i.CreateESService(serviceName)

			time.Sleep(5 * time.Second)

			//reconcile
			pipeline = i.ExpectValidReconcile()

			//validate task is running
			newTasks, err := i.KafkaClient.ListConnectorTasks(pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())
			for _, task := range newTasks {
				Expect(task["state"]).To(Equal("RUNNING"))
			}

			//validate no refresh occurred
			Expect(pipeline.Status.ActivePipelineVersion).To(Equal(activePipelineVersion))
		})

		It("Restarts failed DB connector task without a refresh", func() {
			defer i.ScaleDeployment("inventory-db", "xjoin-operator-project", 1)
			defer test.ForwardPorts()

			pipeline := i.CreateValidPipeline()
			activePipelineVersion := pipeline.Status.ActivePipelineVersion

			//scale down strimzi operator deployment
			i.ScaleDeployment("inventory-db", "xjoin-operator-project", 0)

			//restart task to trigger failure
			tasks, err := i.KafkaClient.ListConnectorTasks(pipeline.Status.ActiveDebeziumConnectorName)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(tasks)).To(BeNumerically(">", 0))
			for _, task := range tasks {
				err = i.KafkaClient.RestartTaskForConnector(pipeline.Status.ActiveDebeziumConnectorName, task["id"].(float64))
				Expect(err).ToNot(HaveOccurred())
			}

			//give connect time to realize the task is failed
			time.Sleep(15 * time.Second)

			//validate task is failed
			tasks, err = i.KafkaClient.ListConnectorTasks(pipeline.Status.ActiveDebeziumConnectorName)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(tasks)).To(BeNumerically(">", 0))
			for _, task := range tasks {
				Expect(task["state"]).To(Equal("FAILED"))
			}

			i.ScaleDeployment("inventory-db", "xjoin-operator-project", 1)
			test.ForwardPorts()

			//reconcile
			pipeline = i.ExpectValidReconcile()

			//validate task is running
			newTasks, err := i.KafkaClient.ListConnectorTasks(pipeline.Status.ActiveDebeziumConnectorName)
			Expect(err).ToNot(HaveOccurred())
			for _, task := range newTasks {
				Expect(task["state"]).To(Equal("RUNNING"))
			}

			//validate no refresh occurred
			Expect(pipeline.Status.ActivePipelineVersion).To(Equal(activePipelineVersion))
		})

		It("Performs a refresh when unable to successfully restart failed connector task", func() {
			serviceName := "xjoin-elasticsearch-es-default-new"
			defer i.ScaleDeployment("strimzi-cluster-operator", "kafka", 1)

			cm := map[string]string{
				"init.validation.attempts.threshold": "1",
				"validation.attempts.threshold":      "1",
			}
			i.CreateConfigMap("xjoin", cm)

			pipeline := i.CreateValidPipeline()

			//give connect time to create the connectors
			time.Sleep(5 * time.Second)

			//scale down strimzi operator deployment
			i.ScaleDeployment("strimzi-cluster-operator", "kafka", 0)

			//give strimzi time to scale down
			time.Sleep(5 * time.Second)

			//update es connector config with invalid url
			i.SetESConnectorURL(
				"http://"+serviceName+":9200",
				pipeline.Status.ActiveESConnectorName)

			//give the connector time to realize it can't connect
			time.Sleep(5 * time.Second)

			//validate task is failed
			tasks, err := i.KafkaClient.ListConnectorTasks(pipeline.Status.ActiveESConnectorName)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(tasks)).To(BeNumerically(">", 0))
			for _, task := range tasks {
				Expect(task["state"]).To(Equal("FAILED"))
			}

			//reconcile
			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_NEW))
			Expect(pipeline.Status.InitialSyncInProgress).To(BeFalse())
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionUnknown))
		})
	})

	Describe("Existing Jenkins Pipeline", func() {
		It("Leaves the existing jenkins pipeline alone during initial sync", func() {
			defer i.cleanupJenkinsResources()
			defer i.setPrefix("xjointest")
			i.createJenkinsResources()

			//set configmap jenkins version
			cm := map[string]string{
				"jenkins.managed.version":              "v1.1",
				"init.validation.percentage.threshold": "0",
				"init.validation.attempts.threshold":   "4",
			}
			i.CreateConfigMap("xjoin", cm)

			//reconcile
			prefix := "xjoin.inventory"
			i.setPrefix(prefix)
			i.CreatePipeline(&xjoin.XJoinPipelineSpec{ResourceNamePrefix: &prefix})
			pipeline := i.ReconcileXJoin()
			version := pipeline.Status.PipelineVersion

			err := i.KafkaClient.PauseElasticSearchConnector(version)
			Expect(err).ToNot(HaveOccurred())

			_ = i.InsertHost()

			pipeline = i.ExpectInitSyncInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Count validation failed - 1 hosts (100.00%) do not match"))

			//validate alias points to jenkins pipeline
			indices, err := i.EsClient.GetCurrentIndicesWithAlias("xjoin.inventory.hosts")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(indices)).To(Equal(1))
			Expect(indices[0]).To(Equal("xjoin.inventory.hosts.v1.1"))

			//validate jenkins managed resources still exist
			i.validateJenkinsResourcesStillExist()
		})

		It("Updates the alias to point to the operator managed pipeline when it becomes valid", func() {
			//create resources to represent existing jenkins pipeline
			defer i.cleanupJenkinsResources()
			defer i.setPrefix("xjointest")
			i.createJenkinsResources()

			//set configmap jenkins version
			cm := map[string]string{
				"jenkins.managed.version":              "v1.1",
				"init.validation.percentage.threshold": "0",
				"init.validation.attempts.threshold":   "4",
			}
			i.CreateConfigMap("xjoin", cm)

			prefix := "xjoin.inventory"
			i.setPrefix(prefix)
			pipeline := i.CreateValidPipeline(&xjoin.XJoinPipelineSpec{ResourceNamePrefix: &prefix})

			//validate alias points to operator managed pipeline
			indices, err := i.EsClient.GetCurrentIndicesWithAlias("xjoin.inventory.hosts")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(indices)).To(Equal(1))
			Expect(indices[0]).To(Equal(pipeline.Status.ActiveIndexName))

			//validate jenkins managed resources still exist
			i.validateJenkinsResourcesStillExist()
		})

		It("Leaves the existing alias alone during failed initial sync", func() {
			defer i.cleanupJenkinsResources()
			defer i.setPrefix("xjointest")
			i.createJenkinsResources()

			//set configmap jenkins version
			cm := map[string]string{
				"jenkins.managed.version":              "v1.1",
				"init.validation.percentage.threshold": "0",
				"init.validation.attempts.threshold":   "2",
			}
			i.CreateConfigMap("xjoin", cm)

			//reconcile
			prefix := "xjoin.inventory"
			i.setPrefix(prefix)
			i.CreatePipeline(&xjoin.XJoinPipelineSpec{ResourceNamePrefix: &prefix})
			pipeline := i.ReconcileXJoin()
			version := pipeline.Status.PipelineVersion

			err := i.KafkaClient.PauseElasticSearchConnector(version)
			Expect(err).ToNot(HaveOccurred())

			_ = i.InsertHost()

			pipeline = i.ExpectInitSyncInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Count validation failed - 1 hosts (100.00%) do not match"))

			i.ExpectNewReconcile()

			//validate alias points to jenkins pipeline
			indices, err := i.EsClient.GetCurrentIndicesWithAlias("xjoin.inventory.hosts")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(indices)).To(Equal(1))
			Expect(indices[0]).To(Equal("xjoin.inventory.hosts.v1.1"))

			//validate jenkins managed resources still exist
			i.validateJenkinsResourcesStillExist()
		})
	})
})
