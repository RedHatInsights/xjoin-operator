package common

import "k8s.io/apimachinery/pkg/runtime/schema"

type Child interface {
	Create(string) error
	Delete(string) error
	GetGVK() schema.GroupVersionKind
	GetInstance() XJoinObject
}
