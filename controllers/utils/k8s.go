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
)

var log = logger.NewLogger("k8s")

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
	err := c.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, secret)

	if statusError, isStatus := err.(*errors.StatusError); isStatus && statusError.Status().Reason == metav1.StatusReasonNotFound {
		log.Info("Secret not found.", "namespace", namespace, "name", name)
		return nil, nil
	}

	return secret, err
}
