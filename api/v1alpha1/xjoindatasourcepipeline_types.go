package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XJoinDataSourcePipelineSpec struct {
	// +kubebuilder:validation:Required
	Version string `json:"version,omitempty"`

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

type XJoinDataSourcePipelineStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=xjoindatasourcepipeline,categories=all

type XJoinDataSourcePipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XJoinDataSourcePipelineSpec   `json:"spec,omitempty"`
	Status XJoinDataSourcePipelineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type XJoinDataSourcePipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []XJoinDataSourcePipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&XJoinDataSourcePipeline{}, &XJoinDataSourcePipelineList{})
}
