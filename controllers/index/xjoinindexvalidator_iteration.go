package index

import (
	"encoding/json"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/avro"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/k8s"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	"github.com/riferrei/srclient"
	v1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
)

const XJoinIndexValidatorFinalizer = "finalizer.xjoin.indexvalidator.cloud.redhat.com"

const ValidatorPodRunning = "running"
const ValidatorPodSuccess = "success"
const ValidatorPodFailed = "failed"

type XJoinIndexValidatorIteration struct {
	common.Iteration
	Parameters             parameters.IndexParameters
	ClientSet              kubernetes.Interface
	ElasticsearchIndexName string
	PodLogReader           k8s.LogReader
}

func (i *XJoinIndexValidatorIteration) Finalize() (err error) {
	i.Log.Info("Starting finalizer")
	controllerutil.RemoveFinalizer(i.Instance, XJoinIndexValidatorFinalizer)

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

	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	//check if pod is already running
	labels := client.MatchingLabels{}
	labels["xjoin.index"] = i.Instance.GetName()
	labels[common.COMPONENT_NAME_LABEL] = "XJoinIndexValidator"
	podList := &v1.PodList{}
	err = i.Client.List(i.Context, podList, client.InNamespace(i.Instance.GetNamespace()), labels)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	//create the pod if not already running
	if len(podList.Items) == 0 {
		dbConnectionEnvVars, err := i.buildDBConnectionEnvVars(indexAvroSchema.References)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}
		err = i.createValidationPod(dbConnectionEnvVars, indexAvroSchema.AvroSchemaString)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}
	}

	//parse the results of the xjoin-validation pod
	pod := &v1.Pod{}
	err = i.Client.Get(i.Context, client.ObjectKey{Name: i.ValidationPodName(), Namespace: i.Instance.GetNamespace()}, pod)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	if pod.Status.Phase == v1.PodSucceeded {
		//check output of xjoin-validation pod
		response, err := i.ParsePodResponse()
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		//update xjoinindexpipeline resource based on xjoin-validation pod's output
		i.Log.Info(response.Message)

		indexNamespacedName := types.NamespacedName{
			Name:      i.Instance.GetOwnerReferences()[0].Name,
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

		return ValidatorPodSuccess, nil
	} else if pod.Status.Phase == v1.PodFailed {
		err = i.Client.Delete(i.Context, pod)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}

		return ValidatorPodFailed, nil
	} else {
		return ValidatorPodRunning, nil
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

	logString, err := i.PodLogReader.GetLogs(i.ValidationPodName(), i.Instance.GetNamespace())
	if err != nil {
		return response, errors.Wrap(err, 0)
	}
	strArray := strings.Split(logString, "\n")

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

func (i *XJoinIndexValidatorIteration) buildDBConnectionEnvVars(references []srclient.Reference) (envVars []v1.EnvVar, err error) {
	//gather db connection info for each datasource
	//the db connection info is passed to the xjoin-validation pod as environment variables
	//because the xjoin-validation pod does not know how to connect to the Kubernetes API
	for _, ref := range references {
		//Get datasourcepipeline k8s object to get db connection info
		dataSourcePipeline := &v1alpha1.XJoinDataSourcePipeline{}
		dataSourcePipelineName := strings.Split(ref.Subject, "xjoindatasourcepipeline.")[1]
		dataSourcePipelineName = strings.Split(dataSourcePipelineName, "-value")[0]
		err = i.Client.Get(
			i.Context,
			client.ObjectKey{Name: dataSourcePipelineName, Namespace: i.GetInstance().Namespace},
			dataSourcePipeline)
		if err != nil {
			return envVars, errors.Wrap(err, 0)
		}

		envVarPrefix := strings.Split(dataSourcePipelineName, ".")[0]
		hostnameEnvVar, err := dataSourcePipeline.Spec.DatabaseHostname.ConvertToEnvVar(envVarPrefix + "_DB_HOSTNAME")
		if err != nil {
			return envVars, errors.Wrap(err, 0)
		}
		envVars = append(envVars, hostnameEnvVar)

		usernameEnvVar, err := dataSourcePipeline.Spec.DatabaseUsername.ConvertToEnvVar(envVarPrefix + "_DB_USERNAME")
		if err != nil {
			return envVars, errors.Wrap(err, 0)
		}
		envVars = append(envVars, usernameEnvVar)

		passwordEnvVar, err := dataSourcePipeline.Spec.DatabasePassword.ConvertToEnvVar(envVarPrefix + "_DB_PASSWORD")
		if err != nil {
			return envVars, errors.Wrap(err, 0)
		}
		envVars = append(envVars, passwordEnvVar)

		nameEnvVar, err := dataSourcePipeline.Spec.DatabaseName.ConvertToEnvVar(envVarPrefix + "_DB_NAME")
		if err != nil {
			return envVars, errors.Wrap(err, 0)
		}
		envVars = append(envVars, nameEnvVar)

		portEnvVar, err := dataSourcePipeline.Spec.DatabasePort.ConvertToEnvVar(envVarPrefix + "_DB_PORT")
		if err != nil {
			return envVars, errors.Wrap(err, 0)
		}
		envVars = append(envVars, portEnvVar)

		tableEnvVar, err := dataSourcePipeline.Spec.DatabaseTable.ConvertToEnvVar(envVarPrefix + "_DB_TABLE")
		if err != nil {
			return envVars, errors.Wrap(err, 0)
		}
		envVars = append(envVars, tableEnvVar)
	}

	return
}

func (i *XJoinIndexValidatorIteration) createValidationPod(dbConnectionEnvVars []v1.EnvVar, fullAvroSchema string) error {
	//run separate xjoin-validation pod
	err := i.Client.Create(i.Context, &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.ValidationPodName(),
			Namespace: i.Instance.GetNamespace(),
			Labels: map[string]string{
				"xjoin.index":               i.Instance.GetName(),
				common.COMPONENT_NAME_LABEL: "XJoinIndexValidator",
			},
			//OwnerReferences: nil,
		},
		Spec: v1.PodSpec{
			RestartPolicy: "Never",
			Containers: []v1.Container{{
				Name:  i.ValidationPodName(),
				Image: "quay.io/cloudservices/xjoin-validation:latest",
				Env: append(dbConnectionEnvVars, []v1.EnvVar{{
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
					Value: i.ElasticsearchIndexName,
				}, {
					Name:  "FULL_AVRO_SCHEMA",
					Value: fullAvroSchema,
				}}...),
				ImagePullPolicy: "Always",
			}},
		},
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}
