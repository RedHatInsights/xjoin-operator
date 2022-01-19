package common

import "k8s.io/apimachinery/pkg/runtime/schema"

var IndexPipelineGVK = schema.GroupVersionKind{
	Group:   "xjoin.cloud.redhat.com",
	Kind:    "XJoinIndexPipeline",
	Version: "v1alpha1",
}

var IndexValidatorGVK = schema.GroupVersionKind{
	Group:   "xjoin.cloud.redhat.com",
	Kind:    "XJoinIndexValidator",
	Version: "v1alpha1",
}

var DataSourceGVK = schema.GroupVersionKind{
	Group:   "xjoin.cloud.redhat.com",
	Kind:    "XJoinDataSource",
	Version: "v1alpha1",
}

var DataSourcePipelineGVK = schema.GroupVersionKind{
	Group:   "xjoin.cloud.redhat.com",
	Kind:    "XJoinDataSourcePipeline",
	Version: "v1alpha1",
}

var DeploymentGVK = schema.GroupVersionKind{
	Group:   "apps",
	Kind:    "Deployment",
	Version: "v1",
}

var ServiceGVK = schema.GroupVersionKind{
	Group:   "",
	Kind:    "Service",
	Version: "v1",
}
