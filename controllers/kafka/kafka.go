package kafka

import (
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Kafka struct {
	Namespace      string
	ConnectCluster string
	KafkaCluster   string
	Owner          *v1alpha1.XJoinPipeline
	OwnerScheme    *runtime.Scheme
	Client         client.Client
}
