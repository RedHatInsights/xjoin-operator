package v1alpha1

import (
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XJoinIndexValidatorSpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`

	// +kubebuilder:validation:Required
	Version string `json:"version,omitempty"`

	// +kubebuilder:validation:Required
	AvroSchema string `json:"avroSchema,omitempty"`

	// +kubebuilder:validation:Required
	IndexName string `json:"indexName,omitempty"`

	// +optional
	Pause bool `json:"pause,omitempty"`
}

type XJoinIndexValidatorStatus struct {
	ValidationResponse validation.ValidationResponse `json:"validationResponse,omitempty"`
	ValidationPodPhase string                        `json:"validationPodPhase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=xjoinindexvalidator,categories=all

type XJoinIndexValidator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XJoinIndexValidatorSpec   `json:"spec,omitempty"`
	Status XJoinIndexValidatorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type XJoinIndexValidatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []XJoinIndexValidator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&XJoinIndexValidator{}, &XJoinIndexValidatorList{})
}
