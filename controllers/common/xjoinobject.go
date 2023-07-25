package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type XJoinObject interface {
	metav1.Object
	runtime.Object
	GetActiveVersion() string
	SetActiveVersion(version string)
	GetActiveVersionState() string
	SetActiveVersionState(state string)
	GetRefreshingVersion() string
	SetRefreshingVersion(version string)
	GetRefreshingVersionState() string
	SetRefreshingVersionState(state string)
	GetSpecHash() string
	GetSpec() interface{}
}
