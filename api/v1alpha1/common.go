package v1alpha1

import (
	"github.com/go-errors/errors"
	v1 "k8s.io/api/core/v1"
)

type StringOrSecretParameter struct {
	// +optional
	Value string `json:"value,omitempty"`

	// +optional
	ValueFrom *SecretKeyRef `json:"valueFrom,omitempty"`
}

func (s StringOrSecretParameter) ConvertToEnvVar(name string) (envVar v1.EnvVar, err error) {
	if name == "" {
		return envVar, errors.Wrap(errors.New("name is required in StringOrSecretParameter.ConvertToEnvVar"), 0)
	}

	envVar.Name = name
	envVar.Value = s.Value
	if s.ValueFrom != nil && s.ValueFrom.SecretKeyRef != nil {
		envVar.ValueFrom = &v1.EnvVarSource{
			SecretKeyRef: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: s.ValueFrom.SecretKeyRef.Name,
				},
				Key:      s.ValueFrom.SecretKeyRef.Key,
				Optional: s.ValueFrom.SecretKeyRef.Optional,
			},
		}
	}

	return
}

type SecretKeyRef struct {
	// +optional
	SecretKeyRef *v1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}
