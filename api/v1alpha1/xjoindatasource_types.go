package v1alpha1

import (
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XJoinDataSourceSpec struct {
	AvroSchema       string                   `json:"avroSchema,omitempty"`
	DatabaseHostname *StringOrSecretParameter `json:"databaseHostname,omitempty"`
	DatabasePort     *StringOrSecretParameter `json:"databasePort,omitempty"`
	DatabaseUsername *StringOrSecretParameter `json:"databaseUsername,omitempty"`
	DatabasePassword *StringOrSecretParameter `json:"databasePassword,omitempty"`
	DatabaseName     *StringOrSecretParameter `json:"databaseName,omitempty"`
	DatabaseTable    *StringOrSecretParameter `json:"databaseTable,omitempty"`

	// +optional
	// +kubebuilder:validation:MinLength:=0
	Refresh string `json:"refresh,omitempty"`

	// +optional
	Pause bool `json:"pause,omitempty"`

	// +optional
	Ephemeral bool `json:"ephemeral,omitempty"`
}

type XJoinDataSourceStatus struct {
	ActiveVersion          string                        `json:"activeVersion"`
	ActiveVersionState     validation.ValidationResponse `json:"activeVersionState"`
	RefreshingVersion      string                        `json:"refreshingVersion"`
	RefreshingVersionState validation.ValidationResponse `json:"refreshingVersionState"`
	SpecHash               string                        `json:"specHash"`
	Conditions             []metav1.Condition            `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=xjoindatasource,categories=all
// +kubebuilder:printcolumn:name="Active_Version",type="string",JSONPath=".status.activeVersion"
// +kubebuilder:printcolumn:name="Active_State",type="string",JSONPath=".status.activeVersionState.result"
// +kubebuilder:printcolumn:name="Refreshing_Version",type="string",JSONPath=".status.refreshingVersion"
// +kubebuilder:printcolumn:name="Refreshing_State",type="string",JSONPath=".status.refreshingVersionState.result"
// +kubebuilder:printcolumn:name="Valid",type="string",JSONPath=".status.conditions[0].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type XJoinDataSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XJoinDataSourceSpec   `json:"spec,omitempty"`
	Status XJoinDataSourceStatus `json:"status,omitempty"`
}

func (in *XJoinDataSource) GetSpec() interface{} {
	return in.Spec
}

func (in *XJoinDataSource) GetSpecHash() string {
	return in.Status.SpecHash
}

func (in *XJoinDataSource) GetActiveVersion() string {
	return in.Status.ActiveVersion
}

func (in *XJoinDataSource) SetActiveVersion(version string) {
	in.Status.ActiveVersion = version
}

func (in *XJoinDataSource) GetActiveVersionState() validation.ValidationResult {
	return in.Status.ActiveVersionState.Result
}

func (in *XJoinDataSource) SetActiveVersionState(state validation.ValidationResult) {
	in.Status.ActiveVersionState.Result = state
}

func (in *XJoinDataSource) GetRefreshingVersion() string {
	return in.Status.RefreshingVersion
}

func (in *XJoinDataSource) SetRefreshingVersion(version string) {
	in.Status.RefreshingVersion = version
}

func (in *XJoinDataSource) GetRefreshingVersionState() validation.ValidationResult {
	return in.Status.RefreshingVersionState.Result
}

func (in *XJoinDataSource) SetRefreshingVersionState(state validation.ValidationResult) {
	in.Status.RefreshingVersionState.Result = state
}

func (in *XJoinDataSource) SetCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&in.Status.Conditions, condition)
}

func (in *XJoinDataSource) GetValidationResult() validation.ValidationResult {
	return in.Status.ActiveVersionState.Result
}

// +kubebuilder:object:root=true

type XJoinDataSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []XJoinDataSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&XJoinDataSource{}, &XJoinDataSourceList{})
}
