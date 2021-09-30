package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XJoinIndexSpec struct {
	// +kubebuilder:validation:Required
	AvroSchema string `json:"avroSchema,omitempty"`

	// +optional
	Pause bool `json:"pause,omitempty"`
}

type XJoinIndexStatus struct {
	ActiveVersion            string `json:"activeVersion"`
	ActiveVersionIsValid     bool   `json:"activeVersionIsValid"`
	RefreshingVersion        string `json:"refreshingVersion"`
	RefreshingVersionIsValid bool   `json:"refreshingVersionIsValid"`
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
