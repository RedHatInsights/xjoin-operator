package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XJoinDataSourceSpec struct {
	AvroSchema string `json:"avroSchema,omitempty"`

	// +optional
	DatabaseHostname *StringOrSecretParameter `json:"databaseHostname,omitempty"`

	// +optional
	DatabasePort *StringOrSecretParameter `json:"databasePort,omitempty"`

	// +optional
	DatabaseUsername *StringOrSecretParameter `json:"databaseUsername,omitempty"`

	// +optional
	DatabasePassword *StringOrSecretParameter `json:"databasePassword,omitempty"`

	// +optional
	DatabaseName *StringOrSecretParameter `json:"databaseName,omitempty"`

	// +optional
	DatabaseTable *StringOrSecretParameter `json:"databaseTable,omitempty"`

	AppName string `json:"appName,omitempty"`

	// +optional
	Pause bool `json:"pause,omitempty"`
}

type XJoinDataSourceStatus struct {
	ActiveVersion            string `json:"activeVersion"`
	ActiveVersionIsValid     bool   `json:"activeVersionIsValid"`
	RefreshingVersion        string `json:"refreshingVersion"`
	RefreshingVersionIsValid bool   `json:"refreshingVersionIsValid"`
	SpecHash                 string `json:"specHash"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=xjoindatasource,categories=all

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

func (in *XJoinDataSource) GetActiveVersionIsValid() bool {
	return in.Status.ActiveVersionIsValid
}

func (in *XJoinDataSource) SetActiveVersionIsValid(valid bool) {
	in.Status.ActiveVersionIsValid = valid
}

func (in *XJoinDataSource) GetRefreshingVersion() string {
	return in.Status.RefreshingVersion
}

func (in *XJoinDataSource) SetRefreshingVersion(version string) {
	in.Status.RefreshingVersion = version
}

func (in *XJoinDataSource) GetRefreshingVersionIsValid() bool {
	return in.Status.RefreshingVersionIsValid
}

func (in *XJoinDataSource) SetRefreshingVersionIsValid(valid bool) {
	in.Status.RefreshingVersionIsValid = valid
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
