package test

import (
	"fmt"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UniqueNamespace(prefix string) (namespace string, err error) {
	namespace = fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = Client.Create(ctx, ns)
	return
}

func StrToBool(str string) (bool, error) {
	return strconv.ParseBool(str)
}

func StrToInt64(str string) (int64, error) {
	return strconv.ParseInt(str, 10, 64)
}
