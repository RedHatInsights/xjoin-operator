package test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Validation controller", func() {
	var i *Iteration

	BeforeEach(func() {
		i = Before()
	})

	AfterEach(func() {
		After(i)
	})

	Describe("Valid pipeline", func() {
		It("Correctly validates fully in-sync table", func() {
			pipeline := i.CreateValidPipeline()
			version := pipeline.Status.PipelineVersion
			i.SyncHosts(version, 3)

			dbCount, err := i.DbClient.CountHosts()
			Expect(err).ToNot(HaveOccurred())
			Expect(dbCount).To(Equal(3))

			esCount, err := i.EsClient.CountIndex(i.EsClient.ESIndexName(version))
			Expect(err).ToNot(HaveOccurred())
			Expect(esCount).To(Equal(3))

			i.ExpectValidReconcile()
		})

		It("Correctly validates fully in-sync initial table", func() {
			i.CreatePipeline()
			pipeline := i.ReconcileXJoin()
			version := pipeline.Status.PipelineVersion

			err := i.KafkaClient.PauseElasticSearchConnector(version)
			Expect(err).ToNot(HaveOccurred())

			i.SyncHosts(version, 3)
			i.ExpectValidReconcile()
		})

		It("Correctly validates initial table after a few tries", func() {
			cm := map[string]string{
				"init.validation.percentage.threshold": "0",
				"init.validation.attempts.threshold":   "4",
			}
			i.CreateConfigMap("xjoin", cm)
			i.CreatePipeline()
			pipeline := i.ReconcileXJoin()
			version := pipeline.Status.PipelineVersion

			err := i.KafkaClient.PauseElasticSearchConnector(version)
			Expect(err).ToNot(HaveOccurred())

			id1 := i.InsertHost()
			id2 := i.InsertHost()
			id3 := i.InsertHost()

			i.ExpectInitSyncInvalidReconcile() // 0/3 synced

			i.IndexDocument(version, id1) // 1/3 synced
			i.ExpectInitSyncInvalidReconcile()

			i.IndexDocument(version, id2) // 2/3 synced
			i.ExpectInitSyncInvalidReconcile()

			i.IndexDocument(version, id3) // 3/3 synced
			i.ExpectValidReconcile()
		})

		It("Correctly validates table after a few tries", func() {
			cm := map[string]string{
				"validation.percentage.threshold": "0",
				"validation.attempts.threshold":   "4",
			}
			i.CreateConfigMap("xjoin", cm)
			i.CreateValidPipeline()
			pipeline := i.ReconcileXJoin()
			version := pipeline.Status.PipelineVersion

			err := i.KafkaClient.PauseElasticSearchConnector(version)
			Expect(err).ToNot(HaveOccurred())

			id1 := i.InsertHost()
			id2 := i.InsertHost()
			id3 := i.InsertHost()

			i.ExpectInvalidReconcile() // 0/3 synced

			i.IndexDocument(version, id1) // 1/3 synced
			i.ExpectInvalidReconcile()

			i.IndexDocument(version, id2) // 2/3 synced
			i.ExpectInvalidReconcile()

			i.IndexDocument(version, id3) // 3/3 synced
			i.ExpectValidReconcile()
		})

		It("Correctly validates pipeline that's slightly off", func() {
			cm := map[string]string{
				"validation.percentage.threshold": "40",
			}
			i.CreateConfigMap("xjoin", cm)
			pipeline := i.CreateValidPipeline()
			version := pipeline.Status.PipelineVersion

			err := i.KafkaClient.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())

			//create 6 hosts in db, 5 in ES
			i.SyncHosts(version, 5)
			i.InsertHost()

			dbCount, err := i.DbClient.CountHosts()
			Expect(err).ToNot(HaveOccurred())
			Expect(dbCount).To(Equal(6))

			esCount, err := i.EsClient.CountIndex(i.EsClient.ESIndexName(version))
			Expect(err).ToNot(HaveOccurred())
			Expect(esCount).To(Equal(5))

			i.ExpectValidReconcile()
		})

		It("Correctly validates initial pipeline that's slightly off", func() {
			cm := map[string]string{
				"init.validation.percentage.threshold": "40",
			}
			i.CreateConfigMap("xjoin", cm)
			i.CreatePipeline()
			pipeline := i.ReconcileXJoin()
			version := pipeline.Status.PipelineVersion

			err := i.KafkaClient.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())

			//create 6 hosts in db, 5 in ES
			i.SyncHosts(version, 5)
			i.InsertHost()

			dbCount, err := i.DbClient.CountHosts()
			Expect(err).ToNot(HaveOccurred())
			Expect(dbCount).To(Equal(6))

			esCount, err := i.EsClient.CountIndex(i.EsClient.ESIndexName(version))
			Expect(err).ToNot(HaveOccurred())
			Expect(esCount).To(Equal(5))

			i.ExpectValidReconcile()
		})
	})

	Describe("Invalid pipeline", func() {
		It("Correctly invalidates pipeline that's way off", func() {
			cm := map[string]string{
				"validation.percentage.threshold": "5",
			}
			i.CreateConfigMap("xjoin", cm)
			pipeline := i.CreateValidPipeline()

			err := i.KafkaClient.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())

			i.InsertHost()
			i.InsertHost()
			i.InsertHost()

			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INVALID))
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))
		})

		It("Correctly invalidates pipeline that's somewhat off", func() {
			cm := map[string]string{
				"validation.percentage.threshold": "19",
				"validation.attempts.threshold":   "1",
			}
			i.CreateConfigMap("xjoin", cm)
			pipeline := i.CreateValidPipeline()
			version := pipeline.Status.PipelineVersion

			err := i.KafkaClient.PauseElasticSearchConnector(version)
			Expect(err).ToNot(HaveOccurred())

			i.InsertHost()
			id1 := i.InsertHost()
			id2 := i.InsertHost()
			id3 := i.InsertHost()
			id4 := i.InsertHost()

			i.IndexDocument(version, id1)
			i.IndexDocument(version, id2)
			i.IndexDocument(version, id3)
			i.IndexDocument(version, id4)

			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INVALID))
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))
		})

		It("Keeps incrementing ValidationFailedCount if failures persist", func() {
			cm := map[string]string{
				"validation.attempts.threshold": "10",
			}
			i.CreateConfigMap("xjoin", cm)
			pipeline := i.CreateValidPipeline()

			err := i.KafkaClient.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())
			i.InsertHost()

			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.ValidationFailedCount).To(Equal(1))
			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.ValidationFailedCount).To(Equal(2))
			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.ValidationFailedCount).To(Equal(3))
		})
	})
})
