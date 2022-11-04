package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XJoinIndexValidatorSpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`

	// +kubebuilder:validation:Required
	Version string `json:"version,omitempty"`

	// +kubebuilder:validation:Required
	AvroSchema string `json:"avroSchema,omitempty"`

	// +optional
	Pause bool `json:"pause,omitempty"`
}

type XJoinIndexValidatorStatus struct {
	ValidationResponse ValidationResponse `json:"validationResponse,omitempty"`
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

type ValidationResponse struct {
	Result  string          `json:"result,omitempty"`
	Reason  string          `json:"reason,omitempty"`
	Message string          `json:"message,omitempty"`
	Details ResponseDetails `json:"details,omitempty"`
}

type ResponseDetails struct {
	TotalMismatch int `json:"totalMismatch,omitempty"`

	IdsMissingFromElasticsearch      []string `json:"idsMissingFromElasticsearch,omitempty"`
	IdsMissingFromElasticsearchCount int      `json:"idsMissingFromElasticsearchCount,omitempty"`

	IdsOnlyInElasticsearch      []string `json:"idsOnlyInElasticsearch,omitempty"`
	IdsOnlyInElasticsearchCount int      `json:"idsOnlyInElasticsearchCount,omitempty"`

	IdsWithMismatchContent []string `json:"idsWithMismatchContent,omitempty"`

	MismatchContentDetails []MismatchContentDetails `json:"mismatchContentDetails,omitempty"`
}

type MismatchContentDetails struct {
	ID                   string `json:"id,omitempty"`
	ElasticsearchContent string `json:"elasticsearchContent,omitempty"`
	DatabaseContent      string `json:"databaseContent,omitempty"`
}

func init() {
	SchemeBuilder.Register(&XJoinIndexValidator{}, &XJoinIndexValidatorList{})
}
