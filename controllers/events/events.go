package events

import (
	"fmt"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

type Events struct {
	recorder record.EventRecorder
	instance runtime.Object
	log      logger.Log
}

func NewEvents(recorder record.EventRecorder, instance runtime.Object, log logger.Log) Events {
	return Events{
		recorder: recorder,
		instance: instance,
		log:      log,
	}
}

func (e Events) Normal(reason, messageFmt string, args ...interface{}) {
	e.log.Info(fmt.Sprintf(reason+": "+messageFmt, args...))
	e.recorder.Eventf(e.instance, corev1.EventTypeNormal, reason, messageFmt, args...)
}

func (e Events) Warning(reason, messageFmt string, args ...interface{}) {
	e.log.Warn(fmt.Sprintf(reason+": "+messageFmt, args...))
	e.recorder.Eventf(e.instance, corev1.EventTypeWarning, reason, messageFmt, args...)
}
