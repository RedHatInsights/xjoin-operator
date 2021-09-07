package utils

import (
	"encoding/json"
	"fmt"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"hash/fnv"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = logger.NewLogger("k8s")

func FetchXJoinPipeline(c client.Client, namespacedName types.NamespacedName) (*xjoin.XJoinPipeline, error) {
	instance := &xjoin.XJoinPipeline{}
	ctx, cancel := DefaultContext()
	defer cancel()
	err := c.Get(ctx, namespacedName, instance)
	return instance, err
}

func FetchXJoinDataSource(c client.Client, namespacedName types.NamespacedName) (*xjoin.XJoinDataSource, error) {
	instance := &xjoin.XJoinDataSource{}
	ctx, cancel := DefaultContext()
	defer cancel()
	err := c.Get(ctx, namespacedName, instance)
	return instance, err
}

func FetchXJoinPipelinesByNamespacedName(c client.Client, name string, namespace string) (*xjoin.XJoinPipelineList, error) {
	nameField := client.MatchingFields{
		"metadata.name":      name,
		"metadata.namespace": namespace,
	}

	list := &xjoin.XJoinPipelineList{}
	ctx, cancel := DefaultContext()
	defer cancel()
	err := c.List(ctx, list, nameField)
	return list, err
}

func FetchXJoinPipelines(c client.Client) (*xjoin.XJoinPipelineList, error) {
	list := &xjoin.XJoinPipelineList{}
	ctx, cancel := DefaultContext()
	defer cancel()
	err := c.List(ctx, list)
	return list, err
}

func FetchConfigMap(c client.Client, namespace string, name string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	ctx, cancel := DefaultContext()
	defer cancel()

	err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, configMap)
	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	return configMap, nil
}

func SecretHash(secret *corev1.Secret) (string, error) {
	if secret == nil {
		return "-1", nil
	}

	jsonVal, err := json.Marshal(secret.Data)

	if err != nil {
		return "-2", nil
	}

	algorithm := fnv.New32a()
	_, err = algorithm.Write(jsonVal)
	return fmt.Sprint(algorithm.Sum32()), err
}

func ConfigMapHash(cm *corev1.ConfigMap, ignoredKeys ...string) (string, error) {
	if cm == nil {
		return "-1", nil
	}

	values := Omit(cm.Data, ignoredKeys...)

	jsonVal, err := json.Marshal(values)

	if err != nil {
		return "-2", nil
	}

	algorithm := fnv.New32a()
	_, err = algorithm.Write(jsonVal)
	return fmt.Sprint(algorithm.Sum32()), err
}

func FetchSecret(c client.Client, namespace string, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	ctx, cancel := DefaultContext()
	defer cancel()
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)

	if statusError, isStatus := err.(*errors.StatusError); isStatus && statusError.Status().Reason == metav1.StatusReasonNotFound {
		log.Info("Secret not found.", "namespace", namespace, "name", name)
		return nil, nil
	}

	return secret, err
}
