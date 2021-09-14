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
	ActiveVersion string `json:"activeVersion"`
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

// +kubebuilder:object:root=true

type XJoinDataSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []XJoinDataSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&XJoinDataSource{}, &XJoinDataSourceList{})
}
