package utils

import (
	"context"
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
	"time"
)

var log = logger.NewLogger("k8s")

func FetchXJoinPipeline(c client.Client, namespacedName types.NamespacedName) (*xjoin.XJoinPipeline, error) {
	instance := &xjoin.XJoinPipeline{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := c.List(ctx, list, nameField)
	return list, err
}

func FetchXJoinPipelines(c client.Client) (*xjoin.XJoinPipelineList, error) {
	list := &xjoin.XJoinPipelineList{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := c.List(ctx, list)
	return list, err
}

func FetchConfigMap(c client.Client, namespace string, name string) (*corev1.ConfigMap, error) {
	nameField := client.MatchingFields{
		"metadata.name":      name,
		"metadata.namespace": namespace,
	}

	configMaps := &corev1.ConfigMapList{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := c.List(ctx, configMaps, nameField)
	if len(configMaps.Items) == 0 {
		emptyConfig := &corev1.ConfigMap{}
		return emptyConfig, nil
	} else {
		return &configMaps.Items[0], err
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)

	if statusError, isStatus := err.(*errors.StatusError); isStatus && statusError.Status().Reason == metav1.StatusReasonNotFound {
		log.Info("Secret not found.", "namespace", namespace, "name", name)
		return nil, nil
	}

	return secret, err
}
