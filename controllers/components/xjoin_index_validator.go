package components

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/common/labels"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type XJoinIndexValidator struct {
	name                   string
	version                string
	indexName              string
	Client                 client.Client
	Context                context.Context
	Namespace              string
	Schema                 string
	Pause                  bool
	ParentInstance         client.Object
	ElasticsearchIndexName string
	Ephemeral              bool
	events                 events.Events
	log                    logger.Log
}

func (xv *XJoinIndexValidator) SetLogger(log logger.Log) {
	xv.log = log
}

func (xv *XJoinIndexValidator) SetName(kind string, name string) {
	xv.indexName = name
	xv.name = strings.ToLower(kind + "." + name)
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
				labels.IndexName:       xv.indexName,
				labels.PipelineVersion: xv.version,
				labels.ComponentName:   IndexValidator,
			},
		},
		"spec": map[string]interface{}{
			"name":       xv.name,
			"version":    xv.version,
			"avroSchema": xv.Schema,
			"pause":      xv.Pause,
			"indexName":  xv.ElasticsearchIndexName,
			"ephemeral":  xv.Ephemeral,
		},
	}
	indexValidator.SetGroupVersionKind(common.IndexValidatorGVK)

	//create child resource
	instanceVal := reflect.ValueOf(xv.ParentInstance).Elem()
	apiVersion := instanceVal.FieldByName("APIVersion")
	if !apiVersion.IsValid() {
		xv.events.Warning("CreateIndexValidatorFailed",
			"Unable to parse APIVersion from parent for IndexValidator %s", xv.Name())
		err = errors.New("status field not found on original instance")
		return errors.Wrap(err, 0)
	}

	kind := instanceVal.FieldByName("Kind")
	if !kind.IsValid() {
		xv.events.Warning("CreateIndexValidatorFailed",
			"Unable to parse Kind from parent for IndexValidator %s", xv.Name())
		err = errors.New("status field not found on original instance")
		return errors.Wrap(err, 0)
	}

	blockOwnerDeletion := true
	controller := true
	ownerReference := metav1.OwnerReference{
		APIVersion:         common.IndexValidatorGVK.Group + "/" + common.IndexValidatorGVK.Version,
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
		xv.events.Warning("CreateIndexValidatorFailed",
			"Unable to create IndexValidator %s", xv.Name())
		return errors.Wrap(err, 0)
	}

	xv.events.Normal("CreatedIndexValidator",
		"Successfully created XJoinIndexValidator %s", xv.Name())
	return
}

func (xv *XJoinIndexValidator) Delete() (err error) {
	indexValidator := &unstructured.Unstructured{}
	indexValidator.SetGroupVersionKind(common.IndexValidatorGVK)
	err = xv.Client.Get(xv.Context, client.ObjectKey{Name: xv.Name(), Namespace: xv.Namespace}, indexValidator)
	if err != nil {
		xv.events.Warning("DeleteIndexValidatorFailed",
			"Unable to get IndexValidator %s", xv.Name())
		return errors.Wrap(err, 0)
	}

	err = xv.Client.Delete(xv.Context, indexValidator)
	if err != nil {
		xv.events.Warning("DeleteIndexValidatorFailed",
			"Unable to delete IndexValidator %s", xv.Name())
		return errors.Wrap(err, 0)
	}

	xv.events.Normal("DeletedIndexValidator",
		"Successfully deleted XJoinIndexValidator %s", xv.Name())
	return
}

func (xv *XJoinIndexValidator) CheckDeviation() (problem, err error) {
	return //TODO checkdeviation for xjoinindexvalidator
}

func (xv *XJoinIndexValidator) Exists() (exists bool, err error) {
	validators := &unstructured.UnstructuredList{}
	validators.SetGroupVersionKind(common.IndexValidatorGVK)
	fields := client.MatchingFields{}
	fields["metadata.name"] = xv.Name()
	labelsMatch := client.MatchingLabels{}
	labelsMatch[labels.ComponentName] = IndexValidator
	labelsMatch[labels.IndexName] = xv.indexName
	labelsMatch[labels.PipelineVersion] = xv.version
	err = xv.Client.List(xv.Context, validators, fields, labelsMatch, client.InNamespace(xv.Namespace))
	if err != nil {
		xv.events.Warning("IndexValidatorExistsFailed",
			"Unable to list IndexValidator deployments %s", xv.Name())
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
	labelsMatch := client.MatchingLabels{}
	labelsMatch[labels.ComponentName] = IndexValidator
	labelsMatch[labels.IndexName] = xv.indexName
	labelsMatch[labels.PipelineVersion] = xv.version
	err = xv.Client.List(xv.Context, validators, labelsMatch, client.InNamespace(xv.Namespace))
	if err != nil {
		xv.events.Warning("IndexValidatorListInstalledVersionsFailed",
			"Unable to list IndexValidator deployments %s", xv.Name())
		return nil, errors.Wrap(err, 0)
	}

	for _, validator := range validators.Items {
		versions = append(versions, validator.GetName())
	}

	return
}

func (xv *XJoinIndexValidator) Reconcile() (err error) {
	return nil
}

func (xv *XJoinIndexValidator) SetEvents(e events.Events) {
	xv.events = e
}
