package v1alpha1

import (
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type XJoinIndexPipelineSpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`

	// +kubebuilder:validation:Required
	Version string `json:"version,omitempty"`

	// +kubebuilder:validation:Required
	AvroSchema string `json:"avroSchema,omitempty"`

	// +optional
	CustomSubgraphImages []CustomSubgraphImage `json:"customSubgraphImages,omitempty"`

	// +optional
	Pause bool `json:"pause,omitempty"`
}

type XJoinIndexPipelineStatus struct {
	ValidationResponse validation.ValidationResponse `json:"validationResponse,omitempty"`
	DataSources        map[string]string             `json:"dataSources,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=xjoinindexpipeline,categories=all

type XJoinIndexPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XJoinIndexPipelineSpec   `json:"spec,omitempty"`
	Status XJoinIndexPipelineStatus `json:"status,omitempty"`
}

func (in *XJoinIndexPipeline) GetDataSources() map[string]string {
	return in.Status.DataSources
}

func (in *XJoinIndexPipeline) GetDataSourceNames() []string {
	keys := make([]string, 0, len(in.Status.DataSources))
	for key := range in.Status.DataSources {
		keys = append(keys, key)
	}
	return keys
}

func (in *XJoinIndexPipeline) GetDataSourcePipelineNames() []string {
	pipelineNames := make([]string, 0, len(in.Status.DataSources))
	for key, value := range in.Status.DataSources {
		pipelineNames = append(pipelineNames, key+"."+value)
	}
	return pipelineNames
}

// +kubebuilder:object:root=true

type XJoinIndexPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []XJoinIndexPipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&XJoinIndexPipeline{}, &XJoinIndexPipelineList{})
}
