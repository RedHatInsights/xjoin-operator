package kafka

import (
	"github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Kafka struct {
	Namespace   string
	Owner       *v1alpha1.XJoinPipeline
	OwnerScheme *runtime.Scheme
	Client      client.Client
	Parameters  config.Parameters
}
