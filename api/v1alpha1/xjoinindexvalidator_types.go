package v1alpha1

import (
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	"k8s.io/apimachinery/pkg/api/meta"
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

	// +optional
	Ephemeral bool `json:"ephemeral,omitempty"`
}

type XJoinIndexValidatorStatus struct {
	ValidationResponse validation.ValidationResponse `json:"validationResponse,omitempty"`
	ValidationPodPhase string                        `json:"validationPodPhase,omitempty"`
	Conditions         []metav1.Condition            `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=xjoinindexvalidator,categories=all
// +kubebuilder:printcolumn:name="PodPhase",type="string",JSONPath=".status.validationPodPhase"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.validationResponse.result"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

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
func (in *XJoinIndexValidator) SetCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&in.Status.Conditions, condition)
}

func (in *XJoinIndexValidator) GetValidationResult() string {
	return in.Status.ValidationResponse.Result
}
