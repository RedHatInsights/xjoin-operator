package components

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type XJoinIndexValidator struct {
	name                   string
	version                string
	Client                 client.Client
	Context                context.Context
	Namespace              string
	Schema                 string
	Pause                  bool
	ParentInstance         client.Object
	ElasticsearchIndexName string
}

func (xv *XJoinIndexValidator) SetName(name string) {
	xv.name = strings.ToLower(name)
}

func (xv *XJoinIndexValidator) SetVersion(version string) {
	xv.version = version
}

func (xv *XJoinIndexValidator) Name() string {
	return xv.name + "." + xv.version
}

func (xv *XJoinIndexValidator) Create() (err error) {
	indexValidator := unstructured.Unstructured{}
	indexValidator.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      xv.Name(),
			"namespace": xv.Namespace,
			"labels": map[string]interface{}{
				common.COMPONENT_NAME_LABEL: xv.name,
				"app":                       "xjoin-validator",
			},
		},
		"spec": map[string]interface{}{
			"name":       xv.name,
			"version":    xv.version,
			"avroSchema": xv.Schema,
			"pause":      xv.Pause,
			"indexName":  xv.ElasticsearchIndexName,
		},
	}
	indexValidator.SetGroupVersionKind(common.IndexValidatorGVK)

	//create child resource
	instanceVal := reflect.ValueOf(xv.ParentInstance).Elem()
	apiVersion := instanceVal.FieldByName("APIVersion")
	if !apiVersion.IsValid() {
		err = errors.New("status field not found on original instance")
		return errors.Wrap(err, 0)
	}

	kind := instanceVal.FieldByName("Kind")
	if !kind.IsValid() {
		err = errors.New("status field not found on original instance")
		return errors.Wrap(err, 0)
	}

	blockOwnerDeletion := true
	controller := true
	ownerReference := metav1.OwnerReference{
		APIVersion:         common.IndexPipelineGVK.Version,
		Kind:               common.IndexPipelineGVK.Kind,
		Name:               xv.ParentInstance.GetName(),
		UID:                xv.ParentInstance.GetUID(),
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
	indexValidator.SetOwnerReferences([]metav1.OwnerReference{ownerReference})

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = xv.Client.Create(ctx, &indexValidator)

	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (xv *XJoinIndexValidator) Delete() (err error) {
	indexValidator := &unstructured.Unstructured{}
	indexValidator.SetGroupVersionKind(common.IndexValidatorGVK)
	err = xv.Client.Get(xv.Context, client.ObjectKey{Name: xv.Name(), Namespace: xv.Namespace}, indexValidator)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	err = xv.Client.Delete(xv.Context, indexValidator)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

func (xv *XJoinIndexValidator) CheckDeviation() (problem error, err error) {
	return
}

func (xv *XJoinIndexValidator) Exists() (exists bool, err error) {
	validators := &unstructured.UnstructuredList{}
	validators.SetGroupVersionKind(common.IndexValidatorGVK)
	fields := client.MatchingFields{}
	fields["metadata.name"] = xv.Name()
	fields["metadata.namespace"] = xv.Namespace
	err = xv.Client.List(xv.Context, validators, fields)
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	if len(validators.Items) > 0 {
		exists = true
	}
	return
}

func (xv *XJoinIndexValidator) ListInstalledVersions() (versions []string, err error) {
	validators := &unstructured.UnstructuredList{}
	validators.SetGroupVersionKind(common.IndexValidatorGVK)
	labels := client.MatchingLabels{}
	labels[common.COMPONENT_NAME_LABEL] = xv.name
	fields := client.MatchingFields{
		"metadata.namespace": xv.Namespace,
	}
	err = xv.Client.List(xv.Context, validators, labels, fields)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	for _, validator := range validators.Items {
		versions = append(versions, validator.GetName())
	}

	return
}
