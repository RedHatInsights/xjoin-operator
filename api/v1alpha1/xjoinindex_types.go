package v1alpha1

import (
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	"k8s.io/apimachinery/pkg/api/meta"
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
	Conditions             []metav1.Condition            `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=xjoinindex,categories=all
// +kubebuilder:printcolumn:name="Active_Version",type="string",JSONPath=".status.activeVersion"
// +kubebuilder:printcolumn:name="Active_State",type="string",JSONPath=".status.activeVersionState.result"
// +kubebuilder:printcolumn:name="Refreshing_Version",type="string",JSONPath=".status.refreshingVersion"
// +kubebuilder:printcolumn:name="Refreshing_State",type="string",JSONPath=".status.refreshingVersionState.result"
// +kubebuilder:printcolumn:name="Valid",type="string",JSONPath=".status.conditions[0].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

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

func (in *XJoinIndex) GetActiveVersionState() validation.ValidationResult {
	return in.Status.ActiveVersionState.Result
}

func (in *XJoinIndex) SetActiveVersionState(state validation.ValidationResult) {
	in.Status.ActiveVersionState.Result = state
}

func (in *XJoinIndex) GetRefreshingVersion() string {
	return in.Status.RefreshingVersion
}

func (in *XJoinIndex) SetRefreshingVersion(version string) {
	in.Status.RefreshingVersion = version
}

func (in *XJoinIndex) GetRefreshingVersionState() validation.ValidationResult {
	return in.Status.RefreshingVersionState.Result
}

func (in *XJoinIndex) SetRefreshingVersionState(state validation.ValidationResult) {
	in.Status.RefreshingVersionState.Result = state
}

func (in *XJoinIndex) SetCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&in.Status.Conditions, condition)
}

func (in *XJoinIndex) GetValidationResult() validation.ValidationResult {
	return in.Status.ActiveVersionState.Result
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
