package test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
)

func UniqueNamespace(prefix string) (namespace string) {
	namespace = fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	err := Client.Create(ctx, ns)
	Expect(err).ToNot(HaveOccurred())

	return
}

func StrToBool(str string) bool {
	b, err := strconv.ParseBool(str)
	Expect(err).ToNot(HaveOccurred())
	return b
}

func StrToInt64(str string) int64 {
	i, err := strconv.ParseInt(str, 10, 64)
	Expect(err).ToNot(HaveOccurred())
	return i
}
