package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XJoinDataSourcePipelineSpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`

	// +kubebuilder:validation:Required
	Version string `json:"version,omitempty"`

	// +kubebuilder:validation:Required
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
