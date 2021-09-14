// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinDataSource) DeepCopyInto(out *XJoinDataSource) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinDataSource.
func (in *XJoinDataSource) DeepCopy() *XJoinDataSource {
	if in == nil {
		return nil
	}
	out := new(XJoinDataSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *XJoinDataSource) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinDataSourceList) DeepCopyInto(out *XJoinDataSourceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]XJoinDataSource, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinDataSourceList.
func (in *XJoinDataSourceList) DeepCopy() *XJoinDataSourceList {
	if in == nil {
		return nil
	}
	out := new(XJoinDataSourceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *XJoinDataSourceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinDataSourcePipeline) DeepCopyInto(out *XJoinDataSourcePipeline) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinDataSourcePipeline.
func (in *XJoinDataSourcePipeline) DeepCopy() *XJoinDataSourcePipeline {
	if in == nil {
		return nil
	}
	out := new(XJoinDataSourcePipeline)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *XJoinDataSourcePipeline) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinDataSourcePipelineList) DeepCopyInto(out *XJoinDataSourcePipelineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]XJoinDataSourcePipeline, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinDataSourcePipelineList.
func (in *XJoinDataSourcePipelineList) DeepCopy() *XJoinDataSourcePipelineList {
	if in == nil {
		return nil
	}
	out := new(XJoinDataSourcePipelineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *XJoinDataSourcePipelineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinDataSourcePipelineSpec) DeepCopyInto(out *XJoinDataSourcePipelineSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinDataSourcePipelineSpec.
func (in *XJoinDataSourcePipelineSpec) DeepCopy() *XJoinDataSourcePipelineSpec {
	if in == nil {
		return nil
	}
	out := new(XJoinDataSourcePipelineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinDataSourcePipelineStatus) DeepCopyInto(out *XJoinDataSourcePipelineStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinDataSourcePipelineStatus.
func (in *XJoinDataSourcePipelineStatus) DeepCopy() *XJoinDataSourcePipelineStatus {
	if in == nil {
		return nil
	}
	out := new(XJoinDataSourcePipelineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinDataSourceSpec) DeepCopyInto(out *XJoinDataSourceSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinDataSourceSpec.
func (in *XJoinDataSourceSpec) DeepCopy() *XJoinDataSourceSpec {
	if in == nil {
		return nil
	}
	out := new(XJoinDataSourceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinDataSourceStatus) DeepCopyInto(out *XJoinDataSourceStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinDataSourceStatus.
func (in *XJoinDataSourceStatus) DeepCopy() *XJoinDataSourceStatus {
	if in == nil {
		return nil
	}
	out := new(XJoinDataSourceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinIndex) DeepCopyInto(out *XJoinIndex) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinIndex.
func (in *XJoinIndex) DeepCopy() *XJoinIndex {
	if in == nil {
		return nil
	}
	out := new(XJoinIndex)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *XJoinIndex) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinIndexList) DeepCopyInto(out *XJoinIndexList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]XJoinIndex, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinIndexList.
func (in *XJoinIndexList) DeepCopy() *XJoinIndexList {
	if in == nil {
		return nil
	}
	out := new(XJoinIndexList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *XJoinIndexList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinIndexSpec) DeepCopyInto(out *XJoinIndexSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinIndexSpec.
func (in *XJoinIndexSpec) DeepCopy() *XJoinIndexSpec {
	if in == nil {
		return nil
	}
	out := new(XJoinIndexSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinIndexStatus) DeepCopyInto(out *XJoinIndexStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinIndexStatus.
func (in *XJoinIndexStatus) DeepCopy() *XJoinIndexStatus {
	if in == nil {
		return nil
	}
	out := new(XJoinIndexStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinPipeline) DeepCopyInto(out *XJoinPipeline) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinPipeline.
func (in *XJoinPipeline) DeepCopy() *XJoinPipeline {
	if in == nil {
		return nil
	}
	out := new(XJoinPipeline)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *XJoinPipeline) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinPipelineList) DeepCopyInto(out *XJoinPipelineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]XJoinPipeline, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinPipelineList.
func (in *XJoinPipelineList) DeepCopy() *XJoinPipelineList {
	if in == nil {
		return nil
	}
	out := new(XJoinPipelineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *XJoinPipelineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinPipelineSpec) DeepCopyInto(out *XJoinPipelineSpec) {
	*out = *in
	if in.ResourceNamePrefix != nil {
		in, out := &in.ResourceNamePrefix, &out.ResourceNamePrefix
		*out = new(string)
		**out = **in
	}
	if in.KafkaCluster != nil {
		in, out := &in.KafkaCluster, &out.KafkaCluster
		*out = new(string)
		**out = **in
	}
	if in.KafkaClusterNamespace != nil {
		in, out := &in.KafkaClusterNamespace, &out.KafkaClusterNamespace
		*out = new(string)
		**out = **in
	}
	if in.ConnectCluster != nil {
		in, out := &in.ConnectCluster, &out.ConnectCluster
		*out = new(string)
		**out = **in
	}
	if in.ConnectClusterNamespace != nil {
		in, out := &in.ConnectClusterNamespace, &out.ConnectClusterNamespace
		*out = new(string)
		**out = **in
	}
	if in.HBIDBSecretName != nil {
		in, out := &in.HBIDBSecretName, &out.HBIDBSecretName
		*out = new(string)
		**out = **in
	}
	if in.ElasticSearchSecretName != nil {
		in, out := &in.ElasticSearchSecretName, &out.ElasticSearchSecretName
		*out = new(string)
		**out = **in
	}
	if in.ElasticSearchNamespace != nil {
		in, out := &in.ElasticSearchNamespace, &out.ElasticSearchNamespace
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinPipelineSpec.
func (in *XJoinPipelineSpec) DeepCopy() *XJoinPipelineSpec {
	if in == nil {
		return nil
	}
	out := new(XJoinPipelineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XJoinPipelineStatus) DeepCopyInto(out *XJoinPipelineStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XJoinPipelineStatus.
func (in *XJoinPipelineStatus) DeepCopy() *XJoinPipelineStatus {
	if in == nil {
		return nil
	}
	out := new(XJoinPipelineStatus)
	in.DeepCopyInto(out)
	return out
}
