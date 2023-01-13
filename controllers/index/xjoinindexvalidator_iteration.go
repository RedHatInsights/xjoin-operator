package index

import (
	"bytes"
	"encoding/json"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/avro"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	"io"
	v1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
)

const xjoinindexvalidatorFinalizer = "finalizer.xjoin.indexvalidator.cloud.redhat.com"

type XJoinIndexValidatorIteration struct {
	common.Iteration
	Parameters parameters.IndexParameters
	ClientSet  *kubernetes.Clientset
}

func (i *XJoinIndexValidatorIteration) Finalize() (err error) {
	i.Log.Info("Starting finalizer")
	controllerutil.RemoveFinalizer(i.Instance, xjoinindexvalidatorFinalizer)

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = i.Client.Update(ctx, i.Instance)
	if err != nil {
		return
	}

	i.Log.Info("Successfully finalized")
	return nil
}

func (i *XJoinIndexValidatorIteration) ReconcileValidationPod() (phase string, err error) {
	//Get index avro schema, references
	registry := schemaregistry.NewSchemaRegistryConfluentClient(
		schemaregistry.ConnectionParams{
			Protocol: i.Parameters.SchemaRegistryProtocol.String(),
			Hostname: i.Parameters.SchemaRegistryHost.String(),
			Port:     i.Parameters.SchemaRegistryPort.String(),
		})
	registry.Init()

	indexAvroSchemaParser := avro.IndexAvroSchemaParser{
		AvroSchema:      i.Parameters.AvroSchema.String(),
		Client:          i.Client,
		Context:         i.Context,
		Namespace:       i.Instance.GetNamespace(),
		Log:             i.Log,
		SchemaRegistry:  registry,
		SchemaNamespace: i.Instance.GetName(),
	}
	indexAvroSchema, err := indexAvroSchemaParser.Parse()

	schemas := make(map[string]string) //map of schema names to schema definition
	var envVars []v1.EnvVar
	for _, ref := range indexAvroSchema.References {
		//Get reference schemas
		refSchema, err := registry.GetSchema(ref.Subject)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}
		schemas[ref.Name] = refSchema

		//Get datasourcepipeline k8s object to get db connection info
		dataSourcePipeline := &v1alpha1.XJoinDataSourcePipeline{}
		dataSourcePipelineName := strings.Split(ref.Subject, "xjoindatasourcepipeline.")[1]
		dataSourcePipelineName = strings.Split(dataSourcePipelineName, "-value")[0]
		err = i.Client.Get(
			i.Context,
			client.ObjectKey{Name: dataSourcePipelineName, Namespace: i.GetInstance().Namespace},
			dataSourcePipeline)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		envVars = append(envVars, v1.EnvVar{
			Name: strings.Split(dataSourcePipelineName, ".")[0] + "_DB_HOSTNAME",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: dataSourcePipeline.Spec.DatabaseHostname.ValueFrom.SecretKeyRef,
			},
		})

		envVars = append(envVars, v1.EnvVar{
			Name: strings.Split(dataSourcePipelineName, ".")[0] + "_DB_USERNAME",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: dataSourcePipeline.Spec.DatabaseUsername.ValueFrom.SecretKeyRef,
			},
		})

		envVars = append(envVars, v1.EnvVar{
			Name: strings.Split(dataSourcePipelineName, ".")[0] + "_DB_PASSWORD",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: dataSourcePipeline.Spec.DatabasePassword.ValueFrom.SecretKeyRef,
			},
		})

		envVars = append(envVars, v1.EnvVar{
			Name: strings.Split(dataSourcePipelineName, ".")[0] + "_DB_NAME",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: dataSourcePipeline.Spec.DatabaseName.ValueFrom.SecretKeyRef,
			},
		})

		envVars = append(envVars, v1.EnvVar{
			Name: strings.Split(dataSourcePipelineName, ".")[0] + "_DB_PORT",
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: dataSourcePipeline.Spec.DatabasePort.ValueFrom.SecretKeyRef,
			},
		})

		envVars = append(envVars, v1.EnvVar{
			Name:  strings.Split(dataSourcePipelineName, ".")[0] + "_DB_TABLE",
			Value: dataSourcePipeline.Spec.DatabaseTable.Value,
		})
	}

	//check if pod is already running
	labels := client.MatchingLabels{}
	labels["xjoin.index"] = i.Instance.GetName()
	podList := &v1.PodList{}
	err = i.Client.List(i.Context, podList, client.InNamespace(i.Instance.GetNamespace()), labels)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	if len(podList.Items) == 0 {
		//run separate xjoin-validation pod
		err = i.Client.Create(i.Context, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      i.ValidationPodName(),
				Namespace: i.Instance.GetNamespace(),
				Labels: map[string]string{
					"xjoin.index": i.Instance.GetName(),
				},
				//OwnerReferences: nil,
			},
			Spec: v1.PodSpec{
				RestartPolicy: "Never",
				Containers: []v1.Container{{
					Name:  i.ValidationPodName(),
					Image: "quay.io/ckyrouac/xjoin-validation:latest",
					Env: append(envVars, []v1.EnvVar{{
						Name: "ELASTICSEARCH_HOST_URL",
						ValueFrom: &v1.EnvVarSource{
							SecretKeyRef: &v1.SecretKeySelector{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "xjoin-elasticsearch",
								},
								Key: "endpoint",
							},
						},
					}, {
						Name: "ELASTICSEARCH_USERNAME",
						ValueFrom: &v1.EnvVarSource{
							SecretKeyRef: &v1.SecretKeySelector{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "xjoin-elasticsearch",
								},
								Key: "username",
							},
						},
					}, {
						Name: "ELASTICSEARCH_PASSWORD",
						ValueFrom: &v1.EnvVarSource{
							SecretKeyRef: &v1.SecretKeySelector{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "xjoin-elasticsearch",
								},
								Key: "password",
							},
						},
					}, {
						Name:  "ELASTICSEARCH_INDEX",
						Value: "",
					}, {
						Name:  "SCHEMA_REGISTRY_PROTOCOL",
						Value: i.Parameters.SchemaRegistryProtocol.String(),
					}, {
						Name:  "SCHEMA_REGISTRY_HOST",
						Value: i.Parameters.SchemaRegistryHost.String(),
					}, {
						Name:  "SCHEMA_REGISTRY_PORT",
						Value: i.Parameters.SchemaRegistryPort.String(),
					}, {
						Name:  "FULL_AVRO_SCHEMA",
						Value: indexAvroSchema.AvroSchemaString,
					}, {
						Name:  "INDEX_AVRO_SCHEMA",
						Value: i.Parameters.AvroSchema.String(),
					}}...),
					ImagePullPolicy: "Always",
				}},
			},
		})
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		return "", nil
	}

	//wait for pod status==completed, retry n times if status==failed
	pod := &v1.Pod{}
	err = i.Client.Get(i.Context, client.ObjectKey{Name: i.ValidationPodName(), Namespace: i.Instance.GetNamespace()}, pod)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	if pod.Status.Phase == "Succeeded" {
		//check output of xjoin-validation pod
		response, err := i.ParsePodResponse()
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		//update xjoinindexpipeline resource based on xjoin-validation pod's output
		i.Log.Info(response.Message)

		indexNamespacedName := types.NamespacedName{
			Name:      i.Instance.GetName(),
			Namespace: i.Instance.GetNamespace(),
		}
		xjoinIndexPipeline, err := k8sUtils.FetchXJoinIndexPipeline(i.Client, indexNamespacedName, i.Context)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}
		xjoinIndexPipeline.Status.ValidationResponse = response

		if err := i.Client.Status().Update(i.Context, xjoinIndexPipeline); err != nil {
			if k8errors.IsConflict(err) {
				i.Log.Error(err, "Status conflict")
				return "", errors.Wrap(err, 0)
			}

			return "", errors.Wrap(err, 0)
		}

		err = i.Client.Delete(i.Context, pod)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		//TODO: delete secret

		return response.Result, nil
	} else if pod.Status.Phase == "Failed" {
		return "failed", nil
	} else {
		return "running", nil
	}
}

func (i *XJoinIndexValidatorIteration) GetInstance() *v1alpha1.XJoinIndexValidator {
	return i.Instance.(*v1alpha1.XJoinIndexValidator)
}

func (i *XJoinIndexValidatorIteration) ValidationPodName() string {
	name := "xjoin-validation-" + i.Instance.GetName()
	name = strings.ReplaceAll(name, ".", "-")
	return name
}

func (i *XJoinIndexValidatorIteration) ParsePodResponse() (validation.ValidationResponse, error) {
	var response validation.ValidationResponse
	podLogOpts := v1.PodLogOptions{}
	req := i.ClientSet.CoreV1().Pods(i.Instance.GetNamespace()).GetLogs(i.ValidationPodName(), &podLogOpts)

	podLogs, err := req.Stream(i.Context)
	if err != nil {
		return response, errors.Wrap(err, 0)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return response, errors.Wrap(err, 0)
	}
	str := buf.String()
	strArray := strings.Split(str, "\n")

	//parse last line, removing trailing new lines
	var resultString string
	if len(strArray) > 1 {
		resultString = strArray[len(strArray)-2]
	} else {
		resultString = strArray[len(strArray)-1]
	}

	err = json.Unmarshal([]byte(resultString), &response)
	if err != nil {
		return response, errors.Wrap(err, 0)
	}
	return response, nil
}
