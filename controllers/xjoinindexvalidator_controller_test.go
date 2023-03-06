package controllers_test

import (
	"context"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/controllers/index"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
			createdIndexValidator := reconciler.ReconcileCreate()
			Expect(createdIndexValidator.Finalizers).To(HaveLen(1))
			Expect(createdIndexValidator.Finalizers).To(ContainElement(index.XJoinIndexValidatorFinalizer))
		})

		It("Should create an xjoin-validation pod", func() {
			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-validator",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
			}
			reconciler.ReconcileCreate()

			connectors := &corev1.PodList{}
			err := k8sClient.List(context.Background(), connectors, client.InNamespace(namespace))
			checkError(err)

			podName := "xjoin-validation-test-index-validator"
			podLookupKey := types.NamespacedName{Name: podName, Namespace: namespace}
			pod := &corev1.Pod{}

			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), podLookupKey, pod)
				return err == nil
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())

		})
	})
})
