package kafka

import (
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Kafka struct {
	Namespace   string
	OwnerScheme *runtime.Scheme
	Client      client.Client
	Parameters  config.Parameters
}
