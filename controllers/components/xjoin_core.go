package components

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type XJoinCore struct {
	name              string
	version           string
	Client            client.Client
	Context           context.Context
	SourceTopics      string
	SinkTopic         string
	KafkaBootstrap    string
	SchemaRegistryURL string
	Namespace         string
}

func (xc *XJoinCore) SetName(name string) {
	xc.name = "xjoin-core-" + strings.ToLower(strings.ReplaceAll(name, ".", "-"))
}

func (xc *XJoinCore) SetVersion(version string) {
	xc.version = version
}

func (xc XJoinCore) Name() string {
	return xc.name + "-" + xc.version
}

func (xc XJoinCore) Create() (err error) {
	deployment := &unstructured.Unstructured{}

	labels := map[string]interface{}{
		"app":         xc.Name(),
		"xjoin.index": xc.name,
	}

	deployment.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      xc.Name(),
			"namespace": xc.Namespace,
			"labels":    labels,
		},
		"spec": map[string]interface{}{
			"replicas": 1,
			"selector": map[string]interface{}{
				"matchLabels": labels,
			},
			"strategy": map[string]interface{}{
				"rollingUpdate": map[string]interface{}{
					"maxSurge":       "25%",
					"maxUnavailable": "25%",
				},
				"type": "RollingUpdate",
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": labels,
				},
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{{
						"env": []map[string]interface{}{
							{
								"name":  "SOURCE_TOPICS",
								"value": xc.SourceTopics,
							},
							{
								"name":  "SINK_TOPIC",
								"value": xc.SinkTopic,
							},
							{
								"name":  "SCHEMA_REGISTRY_URL",
								"value": xc.SchemaRegistryURL,
							},
							{
								"name":  "KAFKA_BOOTSTRAP",
								"value": xc.KafkaBootstrap,
							},
						},
						"image":           "quay.io/ckyrouac/xjoin-core:latest",
						"imagePullPolicy": "Always",
						"name":            xc.Name(),
						"resources": map[string]interface{}{
							"limits": map[string]interface{}{
								"cpu":    "250m",
								"memory": "512Mi",
							},
							"requests": map[string]interface{}{
								"cpu":    "100m",
								"memory": "64Mi",
							},
						},
					}},
				},
			},
		},
	}

	deployment.SetGroupVersionKind(common.DeploymentGVK)

	err = xc.Client.Create(xc.Context, deployment)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return
}

func (xc XJoinCore) Delete() (err error) {
	deployment := &unstructured.Unstructured{}
	deployment.SetGroupVersionKind(common.DeploymentGVK)
	err = xc.Client.Get(xc.Context, client.ObjectKey{Name: xc.Name(), Namespace: xc.Namespace}, deployment)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	err = xc.Client.Delete(xc.Context, deployment)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (xc *XJoinCore) CheckDeviation() (err error) {
	return
}

func (xc XJoinCore) Exists() (exists bool, err error) {
	deployments := &unstructured.UnstructuredList{}
	deployments.SetGroupVersionKind(common.DeploymentGVK)
	fields := client.MatchingFields{}
	fields["metadata.name"] = xc.Name()
	err = xc.Client.List(xc.Context, deployments, fields)
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	if len(deployments.Items) > 0 {
		exists = true
	}
	return
}

func (xc XJoinCore) ListInstalledVersions() (versions []string, err error) {
	deployments := &unstructured.UnstructuredList{}
	deployments.SetGroupVersionKind(common.DeploymentGVK)
	labels := client.MatchingLabels{}
	labels["xjoin.index"] = xc.name
	err = xc.Client.List(xc.Context, deployments, labels)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	for _, deployment := range deployments.Items {
		versions = append(versions, deployment.GetName())
	}

	return
}
