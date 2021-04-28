package test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

var _ = Describe("Validation controller", func() {
	var i *Iteration
	validationSuccessZeroMismatchMessage := "Validation succeeded - 0 hosts IDs (0.00%) do not match, and 0 (0.00%) hosts have inconsistent data."

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
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(validationSuccessZeroMismatchMessage))

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
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(validationSuccessZeroMismatchMessage))
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
				"Count validation failed - 3 hosts (100.00%) do not match"))

			i.IndexDocument(version, id1, "es.document.1")
			pipeline = i.ExpectInitSyncInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Count validation failed - 2 hosts (66.67%) do not match"))

			i.IndexDocument(version, id2, "es.document.1")
			pipeline = i.ExpectInitSyncInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"ID validation failed - 1 hosts (33.33%) do not match"))

			i.IndexDocument(version, id3, "es.document.1")
			pipeline = i.ExpectValidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationSucceeded"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(validationSuccessZeroMismatchMessage))
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
				"Count validation failed - 3 hosts (100.00%) do not match"))

			i.IndexDocument(version, id1, "es.document.1")
			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Count validation failed - 2 hosts (66.67%) do not match"))

			i.IndexDocument(version, id2, "es.document.1")
			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"ID validation failed - 1 hosts (33.33%) do not match"))

			i.IndexDocument(version, id3, "es.document.1")
			pipeline = i.ExpectValidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationSucceeded"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(validationSuccessZeroMismatchMessage))
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
				"Validation succeeded - 1 hosts IDs (16.67%) do not match, and 1 (16.67%) hosts have inconsistent data."))
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
				"Validation succeeded - 1 hosts IDs (16.67%) do not match, and 1 (16.67%) hosts have inconsistent data."))
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
				"Count validation failed - 3 hosts (100.00%) do not match"))
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

			i.IndexDocument(version, id1, "es.document.1")
			i.IndexDocument(version, id2, "es.document.1")
			i.IndexDocument(version, id3, "es.document.1")
			i.IndexDocument(version, id4, "es.document.1")

			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INVALID))
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"ID validation failed - 1 hosts (20.00%) do not match"))
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
				"Count validation failed - 1 hosts (100.00%) do not match"))

			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.ValidationFailedCount).To(Equal(2))
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Count validation failed - 1 hosts (100.00%) do not match"))

			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.ValidationFailedCount).To(Equal(3))
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Count validation failed - 1 hosts (100.00%) do not match"))
		})
	})

	Describe("Full validation", func() {
		It("Performs a full validation when id validation passes", func() {
			cm := map[string]string{
				"validation.percentage.threshold": "20",
			}
			i.CreateConfigMap("xjoin", cm)

			i.CreatePipeline()
			i.ReconcileXJoin()
			pipeline := i.ExpectValidReconcile()
			i.AssertValidationEvents(0)

			for j := 0; j < 5; j++ {
				hostId := i.InsertHost()
				i.IndexDocument(pipeline.Status.PipelineVersion, hostId, "es.document.1")
			}
			i.ReconcileValidation()
			i.AssertValidationEvents(5)
		})

		It("Skips full validation when id validation fails", func() {
			cm := map[string]string{
				"validation.percentage.threshold": "5",
			}
			i.CreateConfigMap("xjoin", cm)
			pipeline := i.CreateValidPipeline()
			i.AssertValidationEvents(0)

			err := i.KafkaClient.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())

			hostId1 := i.InsertHost()
			hostId2 := i.InsertHost()
			i.IndexDocument(pipeline.Status.PipelineVersion, hostId1, "es.document.1")
			i.IndexDocument(pipeline.Status.PipelineVersion, hostId2, "es.document.1")
			i.InsertHost()

			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INVALID))
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))

			recorder, _ := i.ValidationReconciler.Recorder.(*record.FakeRecorder)
			//2 events - one for count validation, one for id validation
			//no event for FullValidation as it should be skipped
			Expect(recorder.Events).To(HaveLen(2))

			//the failure events
			Expect(<-recorder.Events).To(Equal("Normal CountValidationPassed Results: mismatchRatio: 0.3333333333333333, esCount: 2, hbiCount: 3"))
			Expect(<-recorder.Events).To(Equal("Normal IDValidationFailed 1 hosts ids do not match"))
		})

		It("Sets the pipeline invalid when full validation fails", func() {
			cm := map[string]string{
				"validation.percentage.threshold": "5",
			}
			i.CreateConfigMap("xjoin", cm)
			pipeline := i.CreateValidPipeline()
			i.AssertValidationEvents(0)

			err := i.KafkaClient.PauseElasticSearchConnector(pipeline.Status.PipelineVersion)
			Expect(err).ToNot(HaveOccurred())

			hostId1 := i.InsertHost()
			hostId2 := i.InsertHost()
			i.IndexDocument(pipeline.Status.PipelineVersion, hostId1, "es.document.1")
			i.IndexDocument(pipeline.Status.PipelineVersion, hostId2, "es.document.2")

			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INVALID))
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))

			recorder, _ := i.ValidationReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(3))

			//failure events
			msg := <-recorder.Events
			Expect(msg).To(Equal("Normal CountValidationPassed Results: mismatchRatio: 0, esCount: 2, hbiCount: 2"))
			msg = <-recorder.Events
			Expect(msg).To(Equal("Normal IDValidationPassed 0 hosts ids do not match"))
			msg = <-recorder.Events
			Expect(msg).To(Equal("Normal FullValidationFailed 1 hosts do not match. 2 hosts validated."))
		})
	})
})
