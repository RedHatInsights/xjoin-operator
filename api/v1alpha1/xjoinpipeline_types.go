/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// XJoinPipelineSpec defines the desired state of XJoinPipeline
type XJoinPipelineSpec struct {
	// +optional
	// +kubebuilder:validation:MinLength:=3
	ResourceNamePrefix *string `json:"resourceNamePrefix,omitempty"`

	// +optional
	// +kubebuilder:validation:MinLength:=1
	KafkaCluster *string `json:"kafkaCluster,omitempty"`

	// +optional
	// +kubebuilder:validation:MinLength:=1
	ConnectCluster *string `json:"connectCluster,omitempty"`

	// +optional
	// +kubebuilder:validation:MinLength:=1
	ConnectClusterNamespace *string `json:"connectClusterNamespace,omitempty"`

	// +optional
	// +kubebuilder:validation:MinLength:=1
	ElasticSearchSecret *string `json:"elasticSearchURL,omitempty"`

	// +optional
	// +kubebuilder:validation:Min:=0
	// +kubebuilder:validation:Max:=100
	ValidationThreshold *int64 `json:"validationThreshold,omitempty"`

	// +optional
	// +kubebuilder:validation:Min:=0
	MaxAge *int64 `json:"maxAge,omitempty"`

	// +optional
	// +kubebuilder:validation:MinLength:=1
	HBIDBSecretName *string `json:"hbiDBSecretName,omitempty"`

	// +optional
	// +kubebuilder:validation:MinLength:=1
	ElasticSearchSecretName *string `json:"elasticSearchSecretName,omitempty"`
}

// XJoinPipelineStatus defines the observed state of XJoinPipeline
type XJoinPipelineStatus struct {
	PipelineVersion       string             `json:"pipelineVersion"`
	XJoinConfigVersion    string             `json:"xjoinConfigVersion"`
	InitialSyncInProgress bool               `json:"initialSyncInProgress"`
	Conditions            []metav1.Condition `json:"conditions"`
	// +kubebuilder:validation:Minimum:=0
	ValidationFailedCount int `json:"validationFailedCount"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// XJoinPipeline is the Schema for the xjoinpipelines API
type XJoinPipeline struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XJoinPipelineSpec   `json:"spec,omitempty"`
	Status XJoinPipelineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// XJoinPipelineList contains a list of XJoinPipeline
type XJoinPipelineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []XJoinPipeline `json:"items"`
}

func init() {
	SchemeBuilder.Register(&XJoinPipeline{}, &XJoinPipelineList{})
}
