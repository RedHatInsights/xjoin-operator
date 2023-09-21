package common

import (
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type XJoinObject interface {
	metav1.Object
	runtime.Object
	GetActiveVersion() string
	SetActiveVersion(version string)
	GetActiveVersionState() validation.ValidationResult
	SetActiveVersionState(state validation.ValidationResult)
	GetRefreshingVersion() string
	SetRefreshingVersion(version string)
	GetRefreshingVersionState() validation.ValidationResult
	SetRefreshingVersionState(state validation.ValidationResult)
	GetSpecHash() string
	GetSpec() interface{}
	SetCondition(condition metav1.Condition)
}
