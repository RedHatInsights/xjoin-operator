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

			pipeline = i.ExpectValidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationSucceeded"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation succeeded - 0 hosts (0.00%) do not match"))
		})

		It("Correctly validates fully in-sync initial table", func() {
			i.CreatePipeline()
			pipeline := i.ReconcileXJoin()
			version := pipeline.Status.PipelineVersion

			err := i.KafkaClient.PauseElasticSearchConnector(version)
			Expect(err).ToNot(HaveOccurred())

			i.SyncHosts(version, 3)
			pipeline = i.ExpectValidReconcile()

			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationSucceeded"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation succeeded - 0 hosts (0.00%) do not match"))
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

			pipeline = i.ExpectInitSyncInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation failed - 3 hosts (100.00%) do not match"))

			i.IndexDocument(version, id1)
			pipeline = i.ExpectInitSyncInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation failed - 2 hosts (66.67%) do not match"))

			i.IndexDocument(version, id2)
			pipeline = i.ExpectInitSyncInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation failed - 1 hosts (33.33%) do not match"))

			i.IndexDocument(version, id3)
			pipeline = i.ExpectValidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationSucceeded"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation succeeded - 0 hosts (0.00%) do not match"))
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

			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation failed - 3 hosts (100.00%) do not match"))

			i.IndexDocument(version, id1)
			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation failed - 2 hosts (66.67%) do not match"))

			i.IndexDocument(version, id2)
			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation failed - 1 hosts (33.33%) do not match"))

			i.IndexDocument(version, id3)
			pipeline = i.ExpectValidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationSucceeded"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation succeeded - 0 hosts (0.00%) do not match"))
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

			pipeline = i.ExpectValidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationSucceeded"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation succeeded - 1 hosts (16.67%) do not match"))
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

			pipeline = i.ExpectValidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationSucceeded"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation succeeded - 1 hosts (16.67%) do not match"))
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
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation failed - 3 hosts (100.00%) do not match"))
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
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation failed - 1 hosts (20.00%) do not match"))
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
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation failed - 1 hosts (100.00%) do not match"))

			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.ValidationFailedCount).To(Equal(2))
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation failed - 1 hosts (100.00%) do not match"))

			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.ValidationFailedCount).To(Equal(3))
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Validation failed - 1 hosts (100.00%) do not match"))
		})
	})
})
