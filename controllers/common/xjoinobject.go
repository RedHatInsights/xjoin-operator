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
	GetActiveVersionIsValid() bool
	SetActiveVersionIsValid(valid bool)
	GetRefreshingVersion() string
	SetRefreshingVersion(version string)
	GetRefreshingVersionIsValid() bool
	SetRefreshingVersionIsValid(valid bool)
}
