package controllers_test

import (
	"context"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/index"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
			}
			reconciler.ReconcileCreate()

			connectors := &corev1.PodList{}
			err := k8sClient.List(context.Background(), connectors, client.InNamespace(namespace))
			checkError(err)

			podName := "xjoin-validation-" + name
			podLookupKey := types.NamespacedName{Name: podName, Namespace: namespace}
			pod := &corev1.Pod{}

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), podLookupKey, pod)
				return err == nil
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

			Expect(pod.Name).To(Equal("xjoin-validation-" + name))
			Expect(pod.Status.Phase).To(Equal(corev1.PodPending))
			Expect(pod.ObjectMeta.Labels).To(Equal(map[string]string{
				common.COMPONENT_NAME_LABEL: "XJoinIndexValidator",
				"xjoin.index":               name,
			}))

			Expect(pod.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))

			Expect(pod.Spec.Containers).To(HaveLen(1))
			Expect(pod.Spec.Containers[0].Name).To(Equal("xjoin-validation-" + name))
			Expect(pod.Spec.Containers[0].Image).To(Equal("quay.io/ckyrouac/xjoin-validation:latest"))
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
			}
			_, result := reconciler.ReconcileCreate()
			Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 1000000000}))
		})
	})

	Context("Reconcile Pod Running", func() {
		It("Should requeue after ValidationPodStatusInterval while pod is running", func() {
			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-validator",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
			}
			_, result := reconciler.ReconcileRunning()
			Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 1000000000}))
		})
	})

	Context("Reconcile Pod Success", func() {

	})

	Context("Reconcile Pod Failure", func() {

	})
})
