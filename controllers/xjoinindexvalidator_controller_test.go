package controllers_test

import (
	"context"
	"github.com/jarcoal/httpmock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/index"
	"github.com/redhatinsights/xjoin-operator/controllers/k8s/mocks"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
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
				Version:        "1234",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &mocks.LogReader{},
			}
			reconciler.CreateDatasource()
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
				Version:        "1234",
				ConfigFileName: configFileName,
				K8sClient:      k8sClient,
				PodLogReader:   &mocks.LogReader{},
			}
			reconciler.CreateDatasource()
			reconciler.ReconcileCreate()

			pods := reconciler.ListValidatorPods()
			Expect(len(pods.Items)).To(Equal(1))
			pod := pods.Items[0]

			Expect(pod.Name).To(Equal(strings.ReplaceAll(reconciler.GetName(), ".", "-")))
			Expect(pod.Status.Phase).To(Equal(corev1.PodPending))
			Expect(pod.ObjectMeta.Labels).To(Equal(map[string]string{
				common.COMPONENT_NAME_LABEL: "XJoinIndexValidator",
				"xjoin.index":               reconciler.GetName(),
			}))

			Expect(pod.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))

			Expect(pod.Spec.Containers).To(HaveLen(1))
			Expect(pod.Spec.Containers[0].Name).To(Equal(strings.ReplaceAll(reconciler.GetName(), ".", "-")))
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
				Name:      "ELASTICSEARCH_INDEX",
				Value:     "xjoinindexpipeline." + reconciler.GetName(),
				ValueFrom: nil,
			}))

			//avro environment variables
			Expect(pod.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:      "FULL_AVRO_SCHEMA",
				Value:     "{\"type\":\"record\",\"name\":\"Value\",\"namespace\":\"test-index-validator.1234\",\"fields\":[{\"name\":\"testdatasource\",\"type\":{\"name\":\"testdatasource.Value\",\"xjoin.type\":\"reference\"}}]}",
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
				Version:        "1234",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &mocks.LogReader{},
			}
			reconciler.CreateDatasource()
			_, result := reconciler.ReconcileCreate()
			Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 5 * time.Second}))
		})
	})

	Context("Reconcile Pod Running", func() {
		It("Should requeue after ValidationPodStatusInterval", func() {
			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-validator",
				Version:        "1234",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &mocks.LogReader{},
			}
			reconciler.CreateDatasource()
			reconciler.ReconcileCreate()
			validator, result := reconciler.ReconcileRunning()
			Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 5 * time.Second}))
			Expect(validator.Status.ValidationPodPhase).To(Equal(index.ValidatorPodRunning))
		})

		It("Should NOT create more pods while a pod is already running", func() {
			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           "test-index-validator",
				Version:        "1234",
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &mocks.LogReader{},
			}
			reconciler.CreateDatasource()
			reconciler.ReconcileCreate()
			validator, _ := reconciler.ReconcileRunning()
			Expect(validator.Status.ValidationPodPhase).To(Equal(index.ValidatorPodRunning))

			pods := reconciler.ListValidatorPods()
			Expect(len(pods.Items)).To(Equal(1))
		})
	})

	Context("Reconcile Pod Success", func() {
		It("Should requeue after ValidationInterval", func() {
			name := "test-index-validator"
			version := "1234"

			logBytes, err := os.ReadFile("./test/data/validator/success.log.txt")
			checkError(err)

			podLogReader := mocks.LogReader{}
			podLogReader.
				On("GetLogs", name+"-"+version, namespace).Return(string(logBytes), err)

			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           name,
				Version:        version,
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &podLogReader,
			}
			reconciler.CreateDatasource()
			reconciler.ReconcileCreate()
			reconciler.ReconcileRunning()
			validator, result := reconciler.ReconcileSuccess()
			Expect(validator.Status.ValidationPodPhase).To(Equal(index.ValidatorPodSuccess))
			Expect(result).To(Equal(reconcile.Result{Requeue: false, RequeueAfter: 60 * time.Second}))
		})

		It("Should delete the validator pod", func() {
			name := "test-index-validator"
			version := "1234"

			logBytes, err := os.ReadFile("./test/data/validator/success.log.txt")
			checkError(err)

			podLogReader := mocks.LogReader{}
			podLogReader.
				On("GetLogs", name+"-"+version, namespace).Return(string(logBytes), err)

			indexValidatorReconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           name,
				Version:        version,
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &podLogReader,
			}
			indexValidatorReconciler.CreateDatasource()
			indexValidatorReconciler.ReconcileCreate()
			indexValidatorReconciler.ReconcileRunning()

			pods := &corev1.PodList{}
			err = k8sClient.List(context.Background(), pods, client.InNamespace(namespace))
			checkError(err)
			Expect(pods.Items[0].Name).To(Equal(strings.ReplaceAll(indexValidatorReconciler.GetName(), ".", "-")))

			indexValidatorReconciler.ReconcileSuccess()

			pods = &corev1.PodList{}
			err = k8sClient.List(context.Background(), pods, client.InNamespace(namespace))
			checkError(err)

			Expect(len(pods.Items)).To(Equal(0))
		})

		It("Should update each DataSourcePipeline's ValidationResponse status", func() {
			//create datasource
			datasourceReconciler := DatasourceTestReconciler{
				Namespace: namespace,
				Name:      "testdatasource",
				K8sClient: k8sClient,
			}
			datasource := datasourceReconciler.ReconcileNew()

			//create index
			indexReconciler := IndexTestReconciler{
				Namespace:          namespace,
				Name:               "test-index",
				AvroSchemaFileName: "xjoinindex-with-referenced-field",
				K8sClient:          k8sClient,
			}
			createdIndex := indexReconciler.ReconcileNew()

			//reconcile indexpipeline to create the indexpipelinevalidator resource
			reconciler := XJoinIndexPipelineTestReconciler{
				Namespace:          namespace,
				Name:               createdIndex.Name,
				Version:            createdIndex.Status.RefreshingVersion,
				AvroSchemaFileName: "xjoinindex",
				K8sClient:          k8sClient,
				DataSources: []DataSource{{
					Name:                     datasource.Name,
					Version:                  datasource.Status.RefreshingVersion,
					ApiCurioResponseFilename: "datasource-latest-version",
				}},
			}
			reconciler.ReconcileUpdated(UpdatedMocksParams{
				GraphQLSchemaExistingState: "ENABLED",
				GraphQLSchemaNewState:      "DISABLED",
			})

			//assert datasourcepipeline is invalid
			datasourcePipelineLookup := types.NamespacedName{
				Name: datasource.Name + "." + datasource.Status.RefreshingVersion, Namespace: namespace}
			datasourcePipeline := &v1alpha1.XJoinDataSourcePipeline{}
			Eventually(func() bool {
				err := k8sClient.Get(context.Background(), datasourcePipelineLookup, datasourcePipeline)
				return err == nil
			}, K8sGetTimeout, K8sGetInterval).Should(BeTrue())
			Expect(datasourcePipeline.Status.ValidationResponse.Result).To(Equal(""))

			//reconcile indexpipelinevalidator to successful validation
			logBytes, err := os.ReadFile("./test/data/validator/success.log.txt")
			checkError(err)

			podLogReader := mocks.LogReader{}
			podLogReader.
				On("GetLogs",
					"xjoinindexpipeline-"+indexReconciler.Name+"-"+createdIndex.Status.RefreshingVersion,
					namespace).Return(string(logBytes), err)

			validator := &v1alpha1.XJoinIndexValidatorList{}
			err = k8sClient.List(context.Background(), validator, client.InNamespace(namespace))
			checkError(err)

			indexValidatorReconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           "xjoinindexpipeline." + indexReconciler.Name,
				Version:        createdIndex.Status.RefreshingVersion,
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &podLogReader,
			}
			indexValidatorReconciler.ReconcileRunning()
			indexValidatorReconciler.ReconcileSuccess()

			//validate datasourcepipeline is valid
			err = k8sClient.Get(context.Background(), datasourcePipelineLookup, datasourcePipeline)
			checkError(err)
			Expect(datasourcePipeline.Status.ValidationResponse.Result).To(Equal(common.Valid))
		})
	})

	Context("Reconcile Pod Failure", func() {
		It("Should requeue immediately", func() {
			name := "test-index-validator"
			version := "1234"

			logBytes, err := os.ReadFile("./test/data/validator/success.log.txt")
			checkError(err)

			podLogReader := mocks.LogReader{}
			podLogReader.
				On("GetLogs", name+"-"+version, namespace).Return(string(logBytes), err)

			reconciler := XJoinIndexValidatorTestReconciler{
				Namespace:      namespace,
				Name:           name,
				Version:        version,
				ConfigFileName: "xjoinindex",
				K8sClient:      k8sClient,
				PodLogReader:   &podLogReader,
			}
			reconciler.ReconcileCreate()
			_, result := reconciler.ReconcileFailure()
			Expect(result.RequeueAfter).To(Equal(time.Duration(0)))
		})
	})
})
