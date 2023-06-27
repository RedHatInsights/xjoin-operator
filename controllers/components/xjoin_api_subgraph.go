package components

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
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

type XJoinAPISubGraph struct {
	name                  string
	schemaName            string
	version               string
	Client                client.Client
	Context               context.Context
	Namespace             string
	AvroSchema            string
	Registry              *schemaregistry.ConfluentClient
	ElasticSearchUsername string
	ElasticSearchPassword string
	ElasticSearchURL      string
	ElasticSearchIndex    string
	Image                 string
	Suffix                string
	GraphQLSchemaName     string
}

func (x *XJoinAPISubGraph) SetName(kind string, name string) {
	x.schemaName = strings.ToLower(kind + "." + name)
	x.name = strings.ToLower(strings.ReplaceAll(name, ".", "-"))

	if x.Suffix != "" {
		x.name = x.name + "-" + x.Suffix
	}
}

func (x *XJoinAPISubGraph) SetVersion(version string) {
	x.version = version
}

func (x *XJoinAPISubGraph) Name() string {
	return x.name + "-" + x.version
}

func (x *XJoinAPISubGraph) buildServiceStructure() *corev1.Service {
	labels := x.buildLabels()
	targetPort := intstr.Parse("4000")
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      x.Name(),
			Namespace: x.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Protocol:   "TCP",
				Port:       4000,
				TargetPort: targetPort,
			}},
			Selector: map[string]string{
				"app": labels["app"],
			},
		},
		Status: corev1.ServiceStatus{},
	}

	return service
}

func (x *XJoinAPISubGraph) buildDeploymentStructure() (*v1.Deployment, error) {
	labels := x.buildLabels()
	replicas := int32(1)

	cpuLimit, err := resource.ParseQuantity("250m")
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	cpuRequests, err := resource.ParseQuantity("100m")
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	memoryLimit, err := resource.ParseQuantity("512Mi")
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	memoryRequests, err := resource.ParseQuantity("64Mi")
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	container := corev1.Container{
		Name:  x.Name(),
		Image: x.Image,
		Ports: []corev1.ContainerPort{{
			Name:          "web",
			ContainerPort: 8000,
			Protocol:      "TCP",
		}},
		Env: []corev1.EnvVar{{
			Name:  "AVRO_SCHEMA",
			Value: x.AvroSchema,
		}, {
			Name:  "SCHEMA_REGISTRY_PROTOCOL",
			Value: x.Registry.ConnectionParams.Protocol,
		}, {
			Name:  "SCHEMA_REGISTRY_HOSTNAME",
			Value: x.Registry.ConnectionParams.Hostname,
		}, {
			Name:  "SCHEMA_REGISTRY_PORT",
			Value: x.Registry.ConnectionParams.Port,
		}, {
			Name:  "ELASTIC_SEARCH_URL",
			Value: x.ElasticSearchURL,
		}, {
			Name:  "ELASTIC_SEARCH_USERNAME",
			Value: x.ElasticSearchUsername,
		}, {
			Name:  "ELASTIC_SEARCH_PASSWORD",
			Value: x.ElasticSearchPassword,
		}, {
			Name:  "ELASTIC_SEARCH_INDEX",
			Value: x.ElasticSearchIndex,
		}, {
			Name:  "GRAPHQL_SCHEMA_NAME",
			Value: x.GraphQLSchemaName,
		}},
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      x.Name(),
			Namespace: x.Namespace,
			Labels:    labels,
		},
		Spec: v1.DeploymentSpec{
			Replicas:                &replicas,
			RevisionHistoryLimit:    &revisionHistoryLimit,
			ProgressDeadlineSeconds: &progressDeadlineSeconds,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
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
					Labels: labels,
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

func (x *XJoinAPISubGraph) buildLabels() map[string]string {
	return map[string]string{
		"app":         x.Name(),
		"xjoin.index": x.name,
	}
}

func (x *XJoinAPISubGraph) Create() (err error) {
	deployment, err := x.buildDeploymentStructure()
	if err != nil {
		return errors.Wrap(err, 0)
	}

	err = x.Client.Create(x.Context, deployment)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	//create the service
	service := x.buildServiceStructure()
	err = x.Client.Create(x.Context, service)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return
}

func (x *XJoinAPISubGraph) Delete() (err error) {
	//delete the deployment
	deployment := &unstructured.Unstructured{}
	deployment.SetGroupVersionKind(common.DeploymentGVK)
	err = x.Client.Get(x.Context, client.ObjectKey{Name: x.Name(), Namespace: x.Namespace}, deployment)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	err = x.Client.Delete(x.Context, deployment)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	//delete the service
	service := &unstructured.Unstructured{}
	service.SetGroupVersionKind(common.ServiceGVK)
	err = x.Client.Get(x.Context, client.ObjectKey{Name: x.Name(), Namespace: x.Namespace}, service)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	err = x.Client.Delete(x.Context, service)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	//delete the gql schema from registry
	exists, err := x.Registry.CheckIfSchemaVersionExists(x.schemaName+"."+x.version, 1) //TODO

	if exists {
		err = x.Registry.DeleteSchema(x.schemaName + "." + x.version)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return
}

func (x *XJoinAPISubGraph) CheckDeviation() (problem, err error) {
	//build the expected deployment
	expectedDeployment, err := x.buildDeploymentStructure()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	//get the already created (existing) deployment
	found, err := x.Exists()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if !found {
		return fmt.Errorf("the xjoin-api-subgraph deployment named, %s, does not exist", x.Name()), nil
	}

	existingDeployment := &v1.Deployment{}
	existingDeploymentLookup := types.NamespacedName{
		Namespace: x.Namespace,
		Name:      x.Name(),
	}
	err = x.Client.Get(context.Background(), existingDeploymentLookup, existingDeployment)
	if err != nil {
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
		return fmt.Errorf("xjoin-api-subgraph deployment spec has changed: %s", specDiff), nil
	}

	if existingDeployment.GetNamespace() != expectedDeployment.GetNamespace() {
		return fmt.Errorf(
			"xjoin-api-subgraph deployment namespace has changed from: %s to %s",
			expectedDeployment.GetNamespace(),
			existingDeployment.GetNamespace()), nil
	}
	return
}

func (x *XJoinAPISubGraph) Exists() (exists bool, err error) {
	deployments := &unstructured.UnstructuredList{}
	deployments.SetGroupVersionKind(common.DeploymentGVK)
	fields := client.MatchingFields{}
	fields["metadata.name"] = x.Name()
	fields["metadata.namespace"] = x.Namespace
	err = x.Client.List(x.Context, deployments, fields)
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	if len(deployments.Items) > 0 {
		exists = true
	}

	return
}

func (x *XJoinAPISubGraph) ListInstalledVersions() (versions []string, err error) {
	deployments := &unstructured.UnstructuredList{}
	deployments.SetGroupVersionKind(common.DeploymentGVK)
	fields := client.MatchingFields{
		"metadata.namespace": x.Namespace,
	}
	labels := client.MatchingLabels{}
	labels["xjoin.index"] = x.name
	err = x.Client.List(x.Context, deployments, labels, fields)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	for _, deployment := range deployments.Items {
		versions = append(versions, strings.Split(deployment.GetName(), x.name+"-")[1])
	}

	return
}

func (x *XJoinAPISubGraph) Reconcile() (err error) {
	return nil
}
