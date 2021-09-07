package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XJoinDataSourceSpec struct {
	// +kubebuilder:validation:Required
	AvroSchema string `json:"avroSchema,omitempty"`

	// +kubebuilder:validation:Required
	DatabaseHostname string `json:"databaseHostname,omitempty"`

	// +kubebuilder:validation:Required
	DatabasePort string `json:"databasePort,omitempty"`

	// +kubebuilder:validation:Required
	DatabaseUsername string `json:"databaseUsername,omitempty"`

	// +kubebuilder:validation:Required
	DatabasePassword string `json:"databasePassword,omitempty"`

	// +kubebuilder:validation:Required
	DatabaseName string `json:"databaseName,omitempty"`

	// +kubebuilder:validation:Required
	DatabaseTable string `json:"databaseTable,omitempty"`

	// +optional
	Pause bool `json:"pause,omitempty"`
}

type XJoinDataSourceStatus struct {
	// +kubebuilder:validation:Minimum:=0
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
	Items           []XJoinPipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&XJoinDataSource{}, &XJoinDataSourceList{})
}
