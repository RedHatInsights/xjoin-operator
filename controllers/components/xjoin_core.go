package components

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/common/labels"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type XJoinCore struct {
	name              string
	version           string
	indexName         string
	Client            client.Client
	Context           context.Context
	SourceTopics      string
	SinkTopic         string
	KafkaBootstrap    string
	SchemaRegistryURL string
	Namespace         string
	Schema            string
	events            events.Events
	log               logger.Log
	CPURequests       string
	CPULimit          string
	MemoryRequests    string
	MemoryLimit       string
}

func (xc *XJoinCore) SetLogger(log logger.Log) {
	xc.log = log
}

func (xc *XJoinCore) SetName(kind string, name string) {
	xc.indexName = name
	xc.name = "xjoin-core-" + strings.ToLower(strings.ReplaceAll(kind+"-"+name, ".", "-"))
}

func (xc *XJoinCore) SetVersion(version string) {
	xc.version = version
}

func (xc *XJoinCore) Name() string {
	return xc.name + "-" + xc.version
}

func (xc *XJoinCore) buildDeploymentStructure() (*v1.Deployment, error) {
	deploymentLabels := map[string]string{
		labels.IndexName:     xc.indexName,
		labels.ComponentName: Core,
	}

	replicas := int32(1)

	cpuLimit, err := resource.ParseQuantity(xc.CPULimit)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	cpuRequests, err := resource.ParseQuantity(xc.CPURequests)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	memoryLimit, err := resource.ParseQuantity(xc.MemoryLimit)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	memoryRequests, err := resource.ParseQuantity(xc.MemoryRequests)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	container := corev1.Container{
		Name:  xc.Name(),
		Image: "quay.io/cloudservices/xjoin-core:latest",
		Env: []corev1.EnvVar{{
			Name:  "SOURCE_TOPICS",
			Value: xc.SourceTopics,
		}, {
			Name:  "SINK_TOPIC",
			Value: xc.SinkTopic,
		}, {
			Name:  "SCHEMA_REGISTRY_URL",
			Value: xc.SchemaRegistryURL + "/apis/registry/v2",
		}, {
			Name:  "KAFKA_BOOTSTRAP",
			Value: xc.KafkaBootstrap,
		}, {
			Name:  "SINK_SCHEMA",
			Value: xc.Schema,
		}},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/q/health/ready",
					Port: intstr.IntOrString{
						IntVal: 8080,
					},
					Scheme:      "HTTP",
					HTTPHeaders: nil,
				},
			},
			InitialDelaySeconds:           10,
			TimeoutSeconds:                60,
			PeriodSeconds:                 10,
			SuccessThreshold:              1,
			FailureThreshold:              5,
			TerminationGracePeriodSeconds: nil,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/q/health/live",
					Port: intstr.IntOrString{
						IntVal: 8080,
					},
					Scheme:      "HTTP",
					HTTPHeaders: nil,
				},
			},
			InitialDelaySeconds:           10,
			TimeoutSeconds:                60,
			PeriodSeconds:                 10,
			SuccessThreshold:              1,
			FailureThreshold:              5,
			TerminationGracePeriodSeconds: nil,
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memoryLimit,
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuRequests,
				corev1.ResourceMemory: memoryRequests,
			},
		},
		ImagePullPolicy:          corev1.PullAlways,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
	}

	maxUnavailable := intstr.Parse("25%")
	maxSurge := intstr.Parse("25%")
	terminationGracePeriod := int64(30)
	revisionHistoryLimit := int32(10)
	progressDeadlineSeconds := int32(600)

	deployment := v1.Deployment{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      xc.Name(),
			Namespace: xc.Namespace,
			Labels:    deploymentLabels,
		},
		Spec: v1.DeploymentSpec{
			Replicas:                &replicas,
			RevisionHistoryLimit:    &revisionHistoryLimit,
			ProgressDeadlineSeconds: &progressDeadlineSeconds,
			Selector: &metav1.LabelSelector{
				MatchLabels: deploymentLabels,
			},
			Strategy: v1.DeploymentStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &v1.RollingUpdateDeployment{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: deploymentLabels,
				},
				Spec: corev1.PodSpec{
					Containers:                    []corev1.Container{container},
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					DNSPolicy:                     corev1.DNSClusterFirst,
					SchedulerName:                 corev1.DefaultSchedulerName,
				},
			},
		},
	}

	return &deployment, nil
}

func (xc *XJoinCore) Create() (err error) {
	deployment, err := xc.buildDeploymentStructure()
	if err != nil {
		xc.events.Warning("CreateXJoinCoreFailed",
			"Unable to build XJoinCore deployment spec %s", xc.Name())
		return errors.Wrap(err, 0)
	}
	err = xc.Client.Create(xc.Context, deployment)
	if err != nil {
		xc.events.Warning("CreateXJoinCoreFailed",
			"Unable to create XJoinCore deployment %s", xc.Name())
		return errors.Wrap(err, 0)
	}

	xc.events.Normal("CreatedXJoinCore",
		"XJoinCore deployment %s successfully created", xc.Name())
	return
}

func (xc *XJoinCore) Delete() (err error) {
	deployment := &unstructured.Unstructured{}
	deployment.SetGroupVersionKind(common.DeploymentGVK)
	err = xc.Client.Get(xc.Context, client.ObjectKey{Name: xc.Name(), Namespace: xc.Namespace}, deployment)
	if err != nil {
		xc.events.Warning("DeleteXJoinCoreFailed",
			"Unable to get XJoinCore deployment %s", xc.Name())
		return errors.Wrap(err, 0)
	}

	err = xc.Client.Delete(xc.Context, deployment)
	if err != nil {
		xc.events.Warning("DeleteXJoinCoreFailed",
			"Unable to delete XJoinCore deployment %s", xc.Name())
		return errors.Wrap(err, 0)
	}

	xc.events.Normal("DeletedXJoinCore",
		"XJoinCore deployment %s successfully deleted", xc.Name())
	return
}

func (xc *XJoinCore) CheckDeviation() (problem, err error) {
	//build the expected deployment
	expectedDeployment, err := xc.buildDeploymentStructure()
	if err != nil {
		xc.events.Warning("XJoinCoreCheckDeviationFailed",
			"Unable to build expected deployment spec for XJoinCore %s", xc.Name())
		return nil, errors.Wrap(err, 0)
	}

	//get the already created (existing) deployment
	found, err := xc.Exists()
	if err != nil {
		xc.events.Warning("XJoinCoreCheckDeviationFailed",
			"Unable to check if XJoinCore deployment %s exists", xc.Name())
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		xc.events.Warning("XJoinCoreDeviationFound",
			"XJoinCore %s does not exist", xc.Name())
		return fmt.Errorf("the xjoin-core deployment named, %s, does not exist", xc.Name()), nil
	}

	existingDeployment := &v1.Deployment{}
	existingDeploymentLookup := types.NamespacedName{
		Namespace: xc.Namespace,
		Name:      xc.Name(),
	}
	err = xc.Client.Get(context.Background(), existingDeploymentLookup, existingDeployment)
	if err != nil {
		xc.events.Warning("XJoinCoreCheckDeviationFailed",
			"Unable to get XJoinCore deployment %s", xc.Name())
		return nil, errors.Wrap(err, 0)
	}

	//compare
	expectedDeployment.Spec.Template.Spec.SecurityContext = nil //these are automatically set by kubernetes
	existingDeployment.Spec.Template.Spec.SecurityContext = nil

	specDiff := cmp.Diff(
		expectedDeployment.Spec,
		existingDeployment.Spec,
		utils.NumberNormalizer)

	if len(specDiff) > 0 {
		xc.events.Warning("XJoinCoreDeviationFound",
			"XJoinCore %s spec has changed", xc.Name())
		return fmt.Errorf("xjoin-core deployment spec has changed: %s", specDiff), nil
	}

	if existingDeployment.GetNamespace() != expectedDeployment.GetNamespace() {
		xc.events.Warning("XJoinCoreDeviationFound",
			"XJoinCore %s namespace has changed", xc.Name())
		return fmt.Errorf(
			"xjoin-core deployment namespace has changed from: %s to %s",
			expectedDeployment.GetNamespace(),
			existingDeployment.GetNamespace()), nil
	}
	return
}

func (xc *XJoinCore) Exists() (exists bool, err error) {
	deployments := &unstructured.UnstructuredList{}
	deployments.SetGroupVersionKind(common.DeploymentGVK)
	fields := client.MatchingFields{}
	fields["metadata.name"] = xc.Name()
	labelsMatch := client.MatchingLabels{}
	labelsMatch[labels.ComponentName] = Core
	labelsMatch[labels.IndexName] = xc.indexName
	err = xc.Client.List(xc.Context, deployments, fields, labelsMatch, client.InNamespace(xc.Namespace))
	if err != nil {
		xc.events.Warning("XJoinCoreExistsFailed",
			"Unable to list XJoinCore deployments %s", xc.Name())
		return false, errors.Wrap(err, 0)
	}

	if len(deployments.Items) > 0 {
		exists = true
	}
	return
}

func (xc *XJoinCore) ListInstalledVersions() (versions []string, err error) {
	deployments := &unstructured.UnstructuredList{}
	deployments.SetGroupVersionKind(common.DeploymentGVK)
	labelsMatch := client.MatchingLabels{}
	labelsMatch[labels.ComponentName] = Core
	labelsMatch[labels.IndexName] = xc.indexName
	err = xc.Client.List(xc.Context, deployments, labelsMatch, client.InNamespace(xc.Namespace))
	if err != nil {
		xc.events.Warning("XJoinCoreListInstalledVersionsFailed",
			"Unable to list XJoinCore deployments %s", xc.Name())
		return nil, errors.Wrap(err, 0)
	}

	for _, deployment := range deployments.Items {
		versions = append(versions, strings.Split(deployment.GetName(), xc.name+"-")[1])
	}

	return
}

func (xc *XJoinCore) Reconcile() (err error) {
	return nil
}

func (xc *XJoinCore) SetEvents(e events.Events) {
	xc.events = e
}
