package v1alpha1

import (
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
}

type XJoinIndexStatus struct {
	ActiveVersion            string `json:"activeVersion"`
	ActiveVersionIsValid     bool   `json:"activeVersionIsValid"`
	RefreshingVersion        string `json:"refreshingVersion"`
	RefreshingVersionIsValid bool   `json:"refreshingVersionIsValid"`
	SpecHash                 string `json:"specHash"`
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

func (in *XJoinIndex) GetActiveVersionIsValid() bool {
	return in.Status.ActiveVersionIsValid
}

func (in *XJoinIndex) SetActiveVersionIsValid(valid bool) {
	in.Status.ActiveVersionIsValid = valid
}

func (in *XJoinIndex) GetRefreshingVersion() string {
	return in.Status.RefreshingVersion
}

func (in *XJoinIndex) SetRefreshingVersion(version string) {
	in.Status.RefreshingVersion = version
}

func (in *XJoinIndex) GetRefreshingVersionIsValid() bool {
	return in.Status.RefreshingVersionIsValid
}

func (in *XJoinIndex) SetRefreshingVersionIsValid(valid bool) {
	in.Status.RefreshingVersionIsValid = valid
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
