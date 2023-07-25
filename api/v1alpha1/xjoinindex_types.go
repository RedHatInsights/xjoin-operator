package v1alpha1

import (
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XJoinIndexSpec struct {
	// +kubebuilder:validation:Required
	AvroSchema string `json:"avroSchema,omitempty"`

	// +optional
	CustomSubgraphImages []CustomSubgraphImage `json:"customSubgraphImages,omitempty"`

	// +optional
	Pause bool `json:"pause,omitempty"`

	// +optional
	Ephemeral bool `json:"ephemeral,omitempty"`

	// +optional
	// +kubebuilder:validation:MinLength:=0
	Refresh string `json:"refresh,omitempty"`
}

type XJoinIndexStatus struct {
	ActiveVersion          string                        `json:"activeVersion"`
	ActiveVersionState     validation.ValidationResponse `json:"activeVersionState"`
	RefreshingVersion      string                        `json:"refreshingVersion"`
	RefreshingVersionState validation.ValidationResponse `json:"refreshingVersionState"`
	SpecHash               string                        `json:"specHash"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=xjoinindex,categories=all

type XJoinIndex struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XJoinIndexSpec   `json:"spec,omitempty"`
	Status XJoinIndexStatus `json:"status,omitempty"`
}

type CustomSubgraphImage struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	Image string `json:"image"`
}

func (in *XJoinIndex) GetSpec() interface{} {
	return in.Spec
}

func (in *XJoinIndex) GetSpecHash() string {
	return in.Status.SpecHash
}

func (in *XJoinIndex) GetActiveVersion() string {
	return in.Status.ActiveVersion
}

func (in *XJoinIndex) SetActiveVersion(version string) {
	in.Status.ActiveVersion = version
}

func (in *XJoinIndex) GetActiveVersionState() string {
	return in.Status.ActiveVersionState.Result
}

func (in *XJoinIndex) SetActiveVersionState(state string) {
	in.Status.ActiveVersionState.Result = state
}

func (in *XJoinIndex) GetRefreshingVersion() string {
	return in.Status.RefreshingVersion
}

func (in *XJoinIndex) SetRefreshingVersion(version string) {
	in.Status.RefreshingVersion = version
}

func (in *XJoinIndex) GetRefreshingVersionState() string {
	return in.Status.RefreshingVersionState.Result
}

func (in *XJoinIndex) SetRefreshingVersionState(state string) {
	in.Status.RefreshingVersionState.Result = state
}

// +kubebuilder:object:root=true

type XJoinIndexList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []XJoinIndex `json:"items"`
}

func init() {
	SchemeBuilder.Register(&XJoinIndex{}, &XJoinIndexList{})
}
