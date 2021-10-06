package v1alpha1

import v1 "k8s.io/api/core/v1"

type StringOrSecretParameter struct {
	// +optional
	Value string `json:"value,omitempty"`

	// +optional
	ValueFrom *SecretKeyRef `json:"valueFrom,omitempty"`
}

type SecretKeyRef struct {
	// +optional
	SecretKeyRef *v1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}
