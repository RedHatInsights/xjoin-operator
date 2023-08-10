package common

import (
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type XJoinObjectChild interface {
	metav1.Object
	runtime.Object
	SetCondition(condition metav1.Condition)
	GetValidationResult() validation.ValidationResult
}
