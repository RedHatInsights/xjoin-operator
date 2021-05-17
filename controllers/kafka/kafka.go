package kafka

import (
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = logger.NewLogger("kafka")

type Kafka struct {
	Namespace     string
	OwnerScheme   *runtime.Scheme
	Client        client.Client
	Parameters    config.Parameters
	ParametersMap map[string]interface{}
	Recorder      record.EventRecorder
	Test          bool
}
