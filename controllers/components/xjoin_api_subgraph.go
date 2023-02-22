package components

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func (x *XJoinAPISubGraph) SetName(name string) {
	x.schemaName = strings.ToLower(strings.ReplaceAll(name, ".", "-"))
	x.name = x.schemaName

	if x.Suffix != "" {
		x.name = x.name + "-" + x.Suffix
	}
}

func (x *XJoinAPISubGraph) SetVersion(version string) {
	x.version = version
}

func (x XJoinAPISubGraph) Name() string {
	return x.name + "-" + x.version
}

func (x XJoinAPISubGraph) Create() (err error) {
	deployment := &unstructured.Unstructured{}

	labels := map[string]interface{}{
		"app":         x.Name(),
		"xjoin.index": x.name,
	}

	deployment.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      x.Name(),
			"namespace": x.Namespace,
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
						"ports": []map[string]interface{}{
							{
								"containerPort": 8000,
								"name":          "web",
								"protocol":      "TCP",
							},
						},
						"env": []map[string]interface{}{
							{
								"name":  "AVRO_SCHEMA",
								"value": x.AvroSchema,
							},
							{
								"name":  "SCHEMA_REGISTRY_PROTOCOL",
								"value": "http",
							},
							{
								"name":  "SCHEMA_REGISTRY_HOSTNAME",
								"value": "apicurio.test.svc",
							},
							{
								"name":  "SCHEMA_REGISTRY_PORT",
								"value": "1080",
							},
							{
								"name":  "ELASTIC_SEARCH_URL",
								"value": x.ElasticSearchURL,
							},
							{
								"name":  "ELASTIC_SEARCH_USERNAME",
								"value": x.ElasticSearchUsername,
							},
							{
								"name":  "ELASTIC_SEARCH_PASSWORD",
								"value": x.ElasticSearchPassword,
							},
							{
								"name":  "ELASTIC_SEARCH_INDEX",
								"value": x.ElasticSearchIndex,
							},
							{
								"name":  "GRAPHQL_SCHEMA_NAME",
								"value": x.GraphQLSchemaName,
							},
						},
						"image":           x.Image,
						"imagePullPolicy": "Always",
						"name":            x.Name(),
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

	err = x.Client.Create(x.Context, deployment)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	//create the service
	service := &unstructured.Unstructured{}

	service.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      x.Name(),
			"namespace": x.Namespace,
			"labels":    labels,
		},
		"spec": map[string]interface{}{
			"ports": []map[string]interface{}{
				{
					"port":       4000,
					"protocol":   "TCP",
					"targetPort": 4000,
				},
			},
			"selector": map[string]interface{}{
				"app": labels["app"],
			},
		},
	}

	service.SetGroupVersionKind(common.ServiceGVK)
	err = x.Client.Create(x.Context, service)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return
}

func (x XJoinAPISubGraph) Delete() (err error) {
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
	exists, err := x.Registry.CheckIfSchemaVersionExists(x.schemaName+"-"+x.version, 1) //TODO

	if exists {
		err = x.Registry.DeleteSchema(x.schemaName + "-" + x.version)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	return
}

func (x *XJoinAPISubGraph) CheckDeviation() (problem, err error) {
	return
}

func (x XJoinAPISubGraph) Exists() (exists bool, err error) {
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

func (x XJoinAPISubGraph) ListInstalledVersions() (versions []string, err error) {
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
		versions = append(versions, deployment.GetName())
	}

	return
}
