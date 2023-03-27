package controllers_test

import (
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/index"
	"github.com/redhatinsights/xjoin-operator/controllers/k8s/mocks"
	corev1 "k8s.io/api/core/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("XJoinIndexValidator", func() {
	var namespace string

	BeforeEach(func() {
		httpmock.Activate()
		httpmock.RegisterNoResponder(httpmock.InitialTransport.RoundTrip) //disable mocks for unregistered http requests

		var err error
		namespace, err = NewNamespace()
		checkError(err)
	})

	AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	Context("Reconcile Creation", func() {
		It("Should add a finalizer to the XJoinIndexValidator", func() {
			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-validator",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &mocks.LogReader{},
			}
			createdIndexValidator, _ := reconciler.ReconcileCreate()
			Expect(createdIndexValidator.Finalizers).To(HaveLen(1))
			Expect(createdIndexValidator.Finalizers).To(ContainElement(index.XJoinIndexValidatorFinalizer))
		})

		It("Should create an xjoin-validation pod", func() {
			configFileName := "xjoinindex-with-referenced-field"
			name := "test-index-validator"

			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           name,
				ConfigFileName: configFileName,
				K8sClient:      k8sClient,
				PodLogReader:   &mocks.LogReader{},
			}
			reconciler.ReconcileCreate()

			pods := reconciler.ListValidatorPods()
			Expect(len(pods.Items)).To(Equal(1))
			pod := pods.Items[0]

			Expect(pod.Name).To(Equal("xjoin-validation-" + name))
			Expect(pod.Status.Phase).To(Equal(corev1.PodPending))
			Expect(pod.ObjectMeta.Labels).To(Equal(map[string]string{
				common.COMPONENT_NAME_LABEL: "XJoinIndexValidator",
				"xjoin.index":               name,
			}))

			Expect(pod.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))

			Expect(pod.Spec.Containers).To(HaveLen(1))
			Expect(pod.Spec.Containers[0].Name).To(Equal("xjoin-validation-" + name))
			Expect(pod.Spec.Containers[0].Image).To(Equal("quay.io/cloudservices/xjoin-validation:latest"))
			Expect(pod.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullAlways))

			//elasticsearch connection environment variables
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "ELASTICSEARCH_HOST_URL",
				Value: "",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "xjoin-elasticsearch",
						},
						Key:      "endpoint",
						Optional: nil,
					},
				},
			}))
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "ELASTICSEARCH_USERNAME",
				Value: "",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "xjoin-elasticsearch",
						},
						Key:      "username",
						Optional: nil,
					},
				},
			}))
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "ELASTICSEARCH_PASSWORD",
				Value: "",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "xjoin-elasticsearch",
						},
						Key:      "password",
						Optional: nil,
					},
				},
			}))
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:      "ELASTICSEARCH_INDEX",
				Value:     "xjoinindexpipeline.test-index.1234",
				ValueFrom: nil,
			}))

			//avro environment variables
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:      "FULL_AVRO_SCHEMA",
				Value:     "{\"type\":\"record\",\"name\":\"Value\",\"namespace\":\"test-index-validator\",\"fields\":[{\"name\":\"testdatasource\",\"type\":{\"name\":\"xjoindatasourcepipeline.testdatasource.Value\",\"xjoin.type\":\"reference\"}}]}",
				ValueFrom: nil,
			}))

			//database environment variables
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:      "testdatasource_DB_TABLE",
				Value:     "dbTable",
				ValueFrom: nil,
			}))

			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:      "testdatasource_DB_PORT",
				Value:     "8080",
				ValueFrom: nil,
			}))

			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:      "testdatasource_DB_NAME",
				Value:     "dbName",
				ValueFrom: nil,
			}))

			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:      "testdatasource_DB_PASSWORD",
				Value:     "dbPassword",
				ValueFrom: nil,
			}))

			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:      "testdatasource_DB_USERNAME",
				Value:     "dbUsername",
				ValueFrom: nil,
			}))

			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:      "testdatasource_DB_HOSTNAME",
				Value:     "dbHost",
				ValueFrom: nil,
			}))
		})

		It("Should requeue after ValidationPodStatusInterval", func() {
			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-validator",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &mocks.LogReader{},
			}
			_, result := reconciler.ReconcileCreate()
			Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 5 * time.Second}))
		})
	})

	Context("Reconcile Pod Running", func() {
		It("Should requeue after ValidationPodStatusInterval", func() {
			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-validator",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &mocks.LogReader{},
			}
			validator, result := reconciler.ReconcileRunning()
			Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 5 * time.Second}))
			Expect(validator.Status.ValidationPodPhase).To(Equal(index.ValidatorPodRunning))
		})

		It("Should NOT create more pods while a pod is already running", func() {
			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-validator",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &mocks.LogReader{},
			}
			validator, _ := reconciler.ReconcileRunning()
			Expect(validator.Status.ValidationPodPhase).To(Equal(index.ValidatorPodRunning))

			pods := reconciler.ListValidatorPods()
			Expect(len(pods.Items)).To(Equal(1))
		})
	})

	Context("Reconcile Pod Success", func() {
		It("Should requeue after ValidationInterval", func() {
			name := "test-index-validator"

			logBytes, err := os.ReadFile("./test/data/validator/success.log.txt")
			checkError(err)

			podLogReader := mocks.LogReader{}
			podLogReader.
				On("GetLogs", "xjoin-validation-"+name, namespace).Return(string(logBytes), err)

			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           name,
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &podLogReader,
			}
			validator, result := reconciler.ReconcileSuccess()
			Expect(validator.Status.ValidationPodPhase).To(Equal(index.ValidatorPodSuccess))
			Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 60 * time.Second}))
		})

		It("Should delete the validator pod", func() {

		})
	})

	Context("Reconcile Pod Failure", func() {
		It("Should requeue immediately", func() {

		})
	})
})
