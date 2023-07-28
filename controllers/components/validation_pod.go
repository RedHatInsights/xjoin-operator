package components

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

//this is used to scrub orphaned validation pods,
//the primary validation pod management is done by the validaiton controller

type ValidationPod struct {
	name      string
	version   string
	events    events.Events
	log       logger.Log
	Client    client.Client
	Context   context.Context
	Namespace string
}

func (v *ValidationPod) SetLogger(log logger.Log) {
	v.log = log
}

func (v *ValidationPod) SetName(kind string, name string) {
	v.name = strings.ToLower(strings.ReplaceAll(kind+"-"+name, ".", "-"))
}

func (v *ValidationPod) SetVersion(version string) {
	v.version = version
}

func (v *ValidationPod) Name() string {
	return v.name + "-" + v.version
}

func (v *ValidationPod) SetEvents(e events.Events) {
	v.events = e
}

func (v *ValidationPod) Delete() (err error) {
	pod := &unstructured.Unstructured{}
	pod.SetGroupVersionKind(common.PodGVK)
	err = v.Client.Get(v.Context, client.ObjectKey{Name: v.Name(), Namespace: v.Namespace}, pod)
	if err != nil {
		v.events.Warning("DeleteValidationPodFailed",
			"Unable to get Validation pod %s", v.Name())
		return errors.Wrap(err, 0)
	}

	err = v.Client.Delete(v.Context, pod)
	if err != nil {
		v.events.Warning("DeleteValidationPodFailed",
			"Unable to delete Validation pod %s", v.Name())
		return errors.Wrap(err, 0)
	}

	v.events.Normal("DeletedValidationPod",
		"Validation pod %s successfully deleted", v.Name())
	return
}

func (v *ValidationPod) Exists() (exists bool, err error) {
	pods := &unstructured.UnstructuredList{}
	pods.SetGroupVersionKind(common.PodGVK)
	labels := client.MatchingLabels{}
	labels["xjoin.component.name"] = "XJoinIndexValidator"
	fields := client.MatchingFields{
		"metadata.namespace": v.Namespace,
	}
	err = v.Client.List(v.Context, pods, labels, fields)
	if err != nil {
		v.events.Warning("ValidationPodExistsFailed",
			"Unable to check if Validation pod %s exists", v.Name())
		return false, errors.Wrap(err, 0)
	}

	if len(pods.Items) > 0 {
		exists = true
	}
	return
}

func (v *ValidationPod) ListInstalledVersions() (versions []string, err error) {
	pods := &unstructured.UnstructuredList{}
	pods.SetGroupVersionKind(common.PodGVK)
	labels := client.MatchingLabels{}
	labels["xjoin.component.name"] = "XJoinIndexValidator"
	fields := client.MatchingFields{
		"metadata.namespace": v.Namespace,
	}
	err = v.Client.List(v.Context, pods, labels, fields)
	if err != nil {
		v.events.Warning("ValidationPodListInstalledVersionsFailed",
			"Unable to list Validation pods %s", v.Name())
		return nil, errors.Wrap(err, 0)
	}

	for _, pod := range pods.Items {
		podNamePrefix := strings.ReplaceAll(v.name, ".", "-")
		if strings.Index(pod.GetName(), podNamePrefix) == 0 {
			pieces := strings.Split(pod.GetName(), podNamePrefix+"-")
			versions = append(versions, pieces[len(pieces)-1])
		}
	}

	return
}

func (v *ValidationPod) Reconcile() (err error) {
	return nil
}

func (v *ValidationPod) CheckDeviation() (problem, err error) {
	return nil, nil //validation pods are created by the validation controller
}

func (v *ValidationPod) Create() (err error) {
	return nil //validation pods are created by the validation controller
}
