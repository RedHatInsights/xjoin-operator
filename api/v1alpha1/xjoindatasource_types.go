package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XJoinDataSourceSpec struct {
	// +optional
	AvroSchema string `json:"avroSchema,omitempty"`

	// +optional
	DatabaseHostname string `json:"databaseHostname,omitempty"`

	// +optional
	DatabasePort string `json:"databasePort,omitempty"`

	// +optional
	DatabaseUsername string `json:"databaseUsername,omitempty"`

	// +optional
	DatabasePassword string `json:"databasePassword,omitempty"`

	// +optional
	DatabaseName string `json:"databaseName,omitempty"`

	// +optional
	DatabaseTable string `json:"databaseTable,omitempty"`

	// +optional
	Pause bool `json:"pause,omitempty"`
}

type XJoinDataSourceStatus struct {
	ActiveVersion            string `json:"activeVersion"`
	ActiveVersionIsValid     bool   `json:"activeVersionIsValid"`
	RefreshingVersion        string `json:"refreshingVersion"`
	RefreshingVersionIsValid bool   `json:"refreshingVersionIsValid"`
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
