package log

import (
	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Log struct {
	logger logr.Logger
}

func NewLogger(name string, values ...interface{}) Log {
	l := Log{}
	log := logf.Log.WithName(name)
	l.logger = log.WithValues(values...)
	return l
}

func (l Log) Debug(message string, keysAndValues ...interface{}) {
	l.logger.V(1).Info(message, keysAndValues...)
}

func (l Log) Info(message string, keysAndValues ...interface{}) {
	l.logger.Info(message, keysAndValues...)
}

func (l Log) Error(err error, message string, keysAndValues ...interface{}) {
	l.logger.Error(err, message, keysAndValues...)
}

func (l Log) Trace(message string, keysAndValues ...interface{}) {
	l.logger.V(5).Info(message, keysAndValues...)
}
