package log

import (
	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8szap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strings"
)

type Log struct {
	Logger logr.Logger
	name   string
	values []interface{}
}

func NewLogger(name string, values ...interface{}) Log {
	l := Log{}
	l.name = name
	l.values = values
	opts := func(o *k8szap.Options) {
		o.StacktraceLevel = zapcore.ErrorLevel

		devMode := os.Getenv("DEV_MODE")
		if strings.EqualFold(devMode, "true") {
			o.Development = true

			encoderConfig := zap.NewDevelopmentEncoderConfig()
			encoderConfig.StacktraceKey = "trace"
			encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
			xje := xjoinEncoder{
				encoder: zapcore.NewConsoleEncoder(encoderConfig),
			}
			o.Encoder = &xje
		} else {
			o.Development = false

			encoderConfig := zap.NewProductionEncoderConfig()
			encoderConfig.StacktraceKey = "trace"
			encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
			o.Encoder = zapcore.NewJSONEncoder(encoderConfig)
		}

		logLvl := os.Getenv("LOG_LEVEL")
		if strings.EqualFold(logLvl, "DEBUG") {
			o.Level = zap.DebugLevel
		} else if strings.EqualFold(logLvl, "INFO") {
			o.Level = zap.InfoLevel
		} else {
			o.Level = zap.WarnLevel
		}

		//suppress log spam during test runs
		if strings.EqualFold(os.Getenv("TEST_MODE"), "true") {
			o.DestWriter = ginkgo.GinkgoWriter
		}
	}
	logf.SetLogger(k8szap.New(opts))
	l.Logger = logf.Log
	return l
}

func addMetadata(messageKeyValues []interface{}, metaName string, metaValues []interface{}) []interface{} {
	var response []interface{}

	if metaName != "" {
		response = append(response, "log-name")
		response = append(response, metaName)
	}

	if metaValues != nil {
		response = append(response, metaValues...)
	}

	if messageKeyValues != nil {
		response = append(response, messageKeyValues...)
	}

	return response
}

func (l Log) Debug(message string, keysAndValues ...interface{}) {
	l.Logger.V(1).Info(message, addMetadata(keysAndValues, l.name, l.values)...)
}

func (l Log) Info(message string, keysAndValues ...interface{}) {
	l.Logger.Info(message, addMetadata(keysAndValues, l.name, l.values)...)
}

func (l Log) Warn(message string, keysAndValues ...interface{}) {
	l.Logger.V(-1).Info(message, addMetadata(keysAndValues, l.name, l.values)...)
}

func (l Log) Error(err error, message string, keysAndValues ...interface{}) {
	l.Logger.Error(err, message, addMetadata(keysAndValues, l.name, l.values)...)
}
