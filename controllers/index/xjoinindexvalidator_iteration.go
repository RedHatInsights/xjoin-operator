package index

import (
	"bytes"
	"encoding/json"
	validation "github.com/RedHatInsights/xjoin-validation/pkg"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/schemaregistry"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"io"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func (i *XJoinIndexValidatorIteration) Validate() (phase string, err error) {
	//Get index avro schema, references
	registry := schemaregistry.NewSchemaRegistryConfluentClient(
		schemaregistry.ConnectionParams{
			Protocol: i.Parameters.SchemaRegistryProtocol.String(),
			Hostname: i.Parameters.SchemaRegistryHost.String(),
			Port:     i.Parameters.SchemaRegistryPort.String(),
		})
	registry.Init()

	//name+version of IndexValidator is identical to IndexPipeline
	references, err := registry.GetSchemaReferences("xjoinindexpipeline." + i.GetInstance().GetName())
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	schemas := make(map[string]string)                                //map of schema names to schema definition
	dataSourcePipelines := make(map[string]unstructured.Unstructured) //map of schema names to dataSourcePipeline
	for _, ref := range references {
		//Get reference schemas
		refSchema, err := registry.GetSchema(ref.Subject)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}
		schemas[ref.Name] = refSchema

		//Get datasourcepipeline k8s object to get db connection info
		dataSourcePipeline := &unstructured.Unstructured{}
		dataSourcePipeline.SetGroupVersionKind(common.DataSourcePipelineGVK)
		dataSourcePipelineName := strings.Split(ref.Subject, "XJoinDataSourcePipeline.")[1]
		err = i.Client.Get(
			i.Context,
			client.ObjectKey{Name: dataSourcePipelineName, Namespace: i.GetInstance().Namespace},
			dataSourcePipeline)
		if err != nil {
			return "", errors.Wrap(err, 0)
		}
		dataSourcePipelines[ref.Name] = *dataSourcePipeline
	}

	//use avro schema to map database rows, es documents

	//check if pod is already running
	podList := &v1.PodList{}
	labels := client.MatchingLabels{}
	labels["xjoin.index"] = i.Instance.GetName()
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
					//Env:             nil,
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

		//TODO: update xjoinindex resource based on xjoin-validation pod's output
		i.Log.Info(response.Message)
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

func (i *XJoinIndexValidatorIteration) ParsePodResponse() (validation.Response, error) {
	var response validation.Response
	podLogOpts := v1.PodLogOptions{}
	req := i.ClientSet.CoreV1().Pods(i.Instance.GetNamespace()).GetLogs(i.ValidationPodName(), &podLogOpts) //TODO actual pod name

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
	resultString := strArray[len(strArray)-2] //removes trailing new lines
	err = json.Unmarshal([]byte(resultString), &response)
	if err != nil {
		return response, errors.Wrap(err, 0)
	}
	return response, nil
}
