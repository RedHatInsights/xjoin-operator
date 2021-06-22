package test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"time"
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

			now := time.Now().UTC()

			dbCount, err := i.DbClient.CountHosts(now)
			Expect(err).ToNot(HaveOccurred())
			Expect(dbCount).To(Equal(3))

			esCount, err := i.EsClient.CountIndex(i.EsClient.ESIndexName(version), now)
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

			id1 := i.InsertSimpleHost()
			id2 := i.InsertSimpleHost()
			id3 := i.InsertSimpleHost()

			pipeline = i.ExpectInitSyncInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Count validation failed - 3 hosts (100.00%) do not match"))

			i.IndexSimpleDocument(version, id1)
			pipeline = i.ExpectInitSyncInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Count validation failed - 2 hosts (66.67%) do not match"))

			i.IndexSimpleDocument(version, id2)
			pipeline = i.ExpectInitSyncInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"ID validation failed - 1 hosts (33.33%) do not match"))

			i.IndexSimpleDocument(version, id3)
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

			id1 := i.InsertSimpleHost()
			id2 := i.InsertSimpleHost()
			id3 := i.InsertSimpleHost()

			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Count validation failed - 3 hosts (100.00%) do not match"))

			i.IndexSimpleDocument(version, id1)
			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"Count validation failed - 2 hosts (66.67%) do not match"))

			i.IndexSimpleDocument(version, id2)
			pipeline = i.ExpectInvalidReconcile()
			Expect(pipeline.Status.Conditions[0].Reason).To(Equal("ValidationFailed"))
			Expect(pipeline.Status.Conditions[0].Message).To(Equal(
				"ID validation failed - 1 hosts (33.33%) do not match"))

			i.IndexSimpleDocument(version, id3)
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
			i.InsertSimpleHost()

			now := time.Now().UTC()

			dbCount, err := i.DbClient.CountHosts(now)
			Expect(err).ToNot(HaveOccurred())
			Expect(dbCount).To(Equal(6))

			esCount, err := i.EsClient.CountIndex(i.EsClient.ESIndexName(version), now)
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
			i.InsertSimpleHost()

			now := time.Now().UTC()

			dbCount, err := i.DbClient.CountHosts(now)
			Expect(err).ToNot(HaveOccurred())
			Expect(dbCount).To(Equal(6))

			esCount, err := i.EsClient.CountIndex(i.EsClient.ESIndexName(version), now)
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

			i.InsertSimpleHost()
			i.InsertSimpleHost()
			i.InsertSimpleHost()

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

			i.InsertSimpleHost()
			id1 := i.InsertSimpleHost()
			id2 := i.InsertSimpleHost()
			id3 := i.InsertSimpleHost()
			id4 := i.InsertSimpleHost()

			i.IndexSimpleDocument(version, id1)
			i.IndexSimpleDocument(version, id2)
			i.IndexSimpleDocument(version, id3)
			i.IndexSimpleDocument(version, id4)

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
			i.InsertSimpleHost()

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
				hostId := i.InsertSimpleHost()
				i.IndexSimpleDocument(pipeline.Status.PipelineVersion, hostId)
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

			hostId1 := i.InsertSimpleHost()
			hostId2 := i.InsertSimpleHost()
			i.IndexSimpleDocument(pipeline.Status.PipelineVersion, hostId1)
			i.IndexSimpleDocument(pipeline.Status.PipelineVersion, hostId2)
			i.InsertSimpleHost()

			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INVALID))
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))

			recorder, _ := i.ValidationReconciler.Recorder.(*record.FakeRecorder)
			//2 events - one for count validation, one for id validation
			//no event for FullValidation as it should be skipped
			Expect(recorder.Events).To(HaveLen(2))

			//the failure events
			Expect(<-recorder.Events).To(Equal("Normal CountValidationPassed Results: mismatchRatio: 0.3333333333333333, esCount: 2, hbiCount: 3"))
			Expect(<-recorder.Events).To(Equal("Normal IDValidationFailed 1 hosts ids do not match. Number of hosts IDs retrieved: HBI: 3, ES: 2"))
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

			hostId1 := i.InsertSimpleHost()
			hostId2 := i.InsertSimpleHost()
			i.IndexSimpleDocument(pipeline.Status.PipelineVersion, hostId1)
			i.IndexDocumentNow(pipeline.Status.PipelineVersion, hostId2, "display-name-changed")

			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INVALID))
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))

			recorder, _ := i.ValidationReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(3))

			//failure events
			msg := <-recorder.Events
			Expect(msg).To(Equal("Normal CountValidationPassed Results: mismatchRatio: 0, esCount: 2, hbiCount: 2"))
			msg = <-recorder.Events
			Expect(msg).To(Equal("Normal IDValidationPassed 0 hosts ids do not match. Number of hosts IDs retrieved: HBI: 2, ES: 2"))
			msg = <-recorder.Events
			Expect(msg).To(Equal("Normal FullValidationFailed 1 hosts do not match. 2 hosts validated."))
		})

		It("Respects lag compensation parameter", func() {
			cm := map[string]string{
				"validation.lag.compensation.seconds": "10",
			}
			i.CreateConfigMap("xjoin", cm)
			pipeline := i.CreateValidPipeline()
			i.AssertValidationEvents(0)
			i.SyncHosts(pipeline.Status.PipelineVersion, 4)

			//this host should be validated because modified_on is after 10 seconds
			now := time.Now().UTC()
			nowMinus11 := now.Add(-time.Duration(11) * time.Second)
			id := i.InsertHost("simple", nowMinus11)
			i.IndexDocument(pipeline.Status.PipelineVersion, id, "simple", nowMinus11)

			//this host should not be validated because modified on is too recent
			i.InsertHost("simple", now)

			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))

			recorder, _ := i.ValidationReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(3))
			msg := <-recorder.Events
			Expect(msg).To(Equal("Normal CountValidationPassed Results: mismatchRatio: 0, esCount: 5, hbiCount: 5"))
			msg = <-recorder.Events
			Expect(msg).To(Equal("Normal IDValidationPassed 0 hosts ids do not match. Number of hosts IDs retrieved: HBI: 5, ES: 5"))
			msg = <-recorder.Events
			Expect(msg).To(Equal("Normal FullValidationPassed 0 hosts do not match. 5 hosts validated."))
		})

		It("Respects period parameter", func() {
			cm := map[string]string{
				"validation.period.minutes":           "1",
				"validation.lag.compensation.seconds": "1",
			}
			i.CreateConfigMap("xjoin", cm)
			pipeline := i.CreateValidPipeline()
			i.AssertValidationEvents(0)

			//these hosts should not be validated because modified_on is older than a minute
			i.SyncHosts(pipeline.Status.PipelineVersion, 4)

			//this host should be validated because modified_on is less than a minute old
			now := time.Now().UTC()
			nowMinus11 := now.Add(-time.Duration(11) * time.Second)
			id := i.InsertHost("simple", nowMinus11)
			i.IndexDocument(pipeline.Status.PipelineVersion, id, "simple", nowMinus11)

			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))

			recorder, _ := i.ValidationReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(3))
			msg := <-recorder.Events
			Expect(msg).To(Equal("Normal CountValidationPassed Results: mismatchRatio: 0, esCount: 5, hbiCount: 5"))
			msg = <-recorder.Events
			Expect(msg).To(Equal("Normal IDValidationPassed 0 hosts ids do not match. Number of hosts IDs retrieved: HBI: 1, ES: 1"))
			msg = <-recorder.Events
			Expect(msg).To(Equal("Normal FullValidationPassed 0 hosts do not match. 1 hosts validated."))
		})
	})

	Describe("Full validation JSON", func() {
		It("Fails when a top level key is in ES but not in HBI", func() {
			i.fullValidationFailureTest("simple", "systemprofile-top-level-key-added")
		})

		It("Fails when a top level key is in HBI but not in ES", func() {
			i.fullValidationFailureTest("systemprofile-top-level-key-added", "simple")
		})

		It("Fails when a top level key is mismatched", func() {
			i.fullValidationFailureTest("systemprofile-top-level-key-added", "systemprofile-top-level-key-modified")
		})

		It("Fails when a simple array has an extra value", func() {
			i.fullValidationFailureTest("simple", "systemprofile-extra-simple-array-value")
		})

		It("Fails when a simple array has a missing value", func() {
			i.fullValidationFailureTest("simple", "systemprofile-missing-simple-array-value")
		})

		It("Fails when an array of objects has a missing object", func() {
			i.fullValidationFailureTest("simple", "systemprofile-array-of-objects-missing-object")
		})

		It("Fails when an array of objects has an extra object", func() {
			i.fullValidationFailureTest("simple", "systemprofile-array-of-objects-extra-object")
		})

		It("Fails when an array of objects has an extra key in an object", func() {
			i.fullValidationFailureTest("simple", "systemprofile-array-of-objects-extra-key")
		})

		It("Fails when an array of objects has a key missing in an object", func() {
			i.fullValidationFailureTest("simple", "systemprofile-array-of-objects-missing-key")
		})

		It("Fails when an array of objects has a value changed in an object", func() {
			i.fullValidationFailureTest("simple", "systemprofile-array-of-objects-value-changed")
		})

		It("Fails when an object key is added", func() {
			i.fullValidationFailureTest("simple", "systemprofile-object-key-added")
		})

		It("Fails when an object key is removed", func() {
			i.fullValidationFailureTest("simple", "systemprofile-object-key-removed")
		})

		It("Fails when an object value is changed", func() {
			i.fullValidationFailureTest("simple", "systemprofile-object-value-changed")
		})

		It("Fails when an object is missing", func() {
			i.fullValidationFailureTest("simple", "systemprofile-object-missing")
		})

		It("Fails when an array within an object is modified", func() {
			i.fullValidationFailureTest("simple", "systemprofile-object-array-value-changed")
		})

		It("Fails when a boolean value is modified", func() {
			i.fullValidationFailureTest("simple", "systemprofile-boolean-modified")
		})

		It("Fails when an integer value is modified", func() {
			i.fullValidationFailureTest("simple", "systemprofile-integer-modified")
		})

		It("Fails when there are multiple changes to a single host", func() {
			pipeline := i.CreateValidPipeline()
			i.AssertValidationEvents(0)

			i.SyncHosts(pipeline.Status.PipelineVersion, 5)

			hostId := i.InsertSimpleHost()
			i.IndexDocumentNow(pipeline.Status.PipelineVersion, hostId, "lots-of-changes")

			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_INVALID))
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionFalse))

			recorder, _ := i.ValidationReconciler.Recorder.(*record.FakeRecorder)
			Expect(recorder.Events).To(HaveLen(3))

			msg := <-recorder.Events
			Expect(msg).To(Equal("Normal CountValidationPassed Results: mismatchRatio: 0, esCount: 6, hbiCount: 6"))
			msg = <-recorder.Events
			Expect(msg).To(Equal("Normal IDValidationPassed 0 hosts ids do not match. Number of hosts IDs retrieved: HBI: 6, ES: 6"))
			msg = <-recorder.Events
			Expect(msg).To(Equal("Normal FullValidationFailed 1 hosts do not match. 6 hosts validated."))
		})
	})

	Describe("Tag validation", func() {
		It("Fails when tag structured namespace is inconsistent", func() {
			i.fullValidationFailureTest("simple", "tags-structured-namespace-modified")
		})

		It("Fails when tag structured key is inconsistent", func() {
			i.fullValidationFailureTest("simple", "tags-structured-key-modified")
		})

		It("Fails when tag structured value is inconsistent", func() {
			i.fullValidationFailureTest("simple", "tags-structured-value-modified")
		})

		It("Fails when tag string inconsistent", func() {
			i.fullValidationFailureTest("simple", "tags-string-modified")
		})

		It("Validates complex, unordered tags", func() {
			pipeline := i.CreateValidPipeline()
			i.AssertValidationEvents(0)

			hostId := i.InsertHostNow("tags-multiple-unordered")
			i.IndexDocumentNow(pipeline.Status.PipelineVersion, hostId, "tags-multiple-unordered")

			pipeline = i.ReconcileValidation()
			Expect(pipeline.GetState()).To(Equal(xjoin.STATE_VALID))
			Expect(pipeline.GetValid()).To(Equal(metav1.ConditionTrue))
		})
	})
})
