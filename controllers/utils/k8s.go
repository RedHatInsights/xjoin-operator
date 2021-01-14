package utils

import (
	"context"
	"encoding/json"
	"fmt"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"hash/fnv"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func FetchXJoinPipeline(c client.Client, namespacedName types.NamespacedName) (*xjoin.XJoinPipeline, error) {
	instance := &xjoin.XJoinPipeline{}
	err := c.Get(context.TODO(), namespacedName, instance)
	return instance, err
}

func FetchConfigMap(c client.Client, namespace string, name string) (*corev1.ConfigMap, error) {
	nameField := client.MatchingFields{
		"metadata.name":      name,
		"metadata.namespace": namespace,
	}

	configMaps := &corev1.ConfigMapList{}
	err := c.List(context.TODO(), configMaps, nameField)
	if len(configMaps.Items) == 0 {
		emptyConfig := &corev1.ConfigMap{}
		return emptyConfig, nil
	} else {
		return &configMaps.Items[0], err
	}
}

func ConfigMapHash(cm *corev1.ConfigMap, ignoredKeys ...string) string {
	if cm == nil {
		return "-1"
	}

	values := Omit(cm.Data, ignoredKeys...)

	json, err := json.Marshal(values)

	if err != nil {
		return "-2"
	}

	algorithm := fnv.New32a()
	algorithm.Write(json)
	return fmt.Sprint(algorithm.Sum32())
}

func FetchSecret(c client.Client, namespace string, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := c.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, secret)
	return secret, err
}
