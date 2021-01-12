package controllers

import (
	"context"
	"fmt"
	"github.com/google/go-cmp/cmp"
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/database"
	"github.com/redhatinsights/xjoin-operator/controllers/elasticsearch"
	"github.com/redhatinsights/xjoin-operator/controllers/kafka"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"time"
)

type ReconcileIteration struct {
	Instance *xjoin.XJoinPipeline
	// Do not alter this copy
	// Used for tracking of whether Reconcile actually changed the state or not
	OriginalInstance *xjoin.XJoinPipeline

	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
	Log      logger.Log
	Client   client.Client
	Now      string

	config config.XJoinConfiguration

	ESClient    *elasticsearch.ElasticSearch
	Kafka       kafka.Kafka
	InventoryDb database.Database

	GetRequeueInterval func(i *ReconcileIteration) (result int)
}

func (i *ReconcileIteration) Close() {
	if i.InventoryDb != nil {
		i.InventoryDb.Close()
	}
}

// logs the error and produces an error log message
func (i *ReconcileIteration) error(err error, prefixes ...string) {
	msg := err.Error()

	if len(prefixes) > 0 {
		prefix := strings.Join(prefixes[:], ", ")
		msg = fmt.Sprintf("%s: %s", prefix, msg)
	}

	i.Log.Error(err, msg)

	i.eventWarning("Failed", msg)
}

func (i *ReconcileIteration) eventNormal(reason, messageFmt string, args ...interface{}) {
	i.Recorder.Eventf(i.Instance, corev1.EventTypeNormal, reason, messageFmt, args...)
}

func (i *ReconcileIteration) eventWarning(reason, messageFmt string, args ...interface{}) {
	i.Recorder.Eventf(i.Instance, corev1.EventTypeWarning, reason, messageFmt, args...)
}

func (i *ReconcileIteration) debug(message string, keysAndValues ...interface{}) {
	i.Log.Debug(message, keysAndValues...)
}

func (i *ReconcileIteration) updateStatusAndRequeue() (reconcile.Result, error) {
	// Only issue status update if Reconcile actually modified Status
	// This prevents write conflicts between the controllers
	if !cmp.Equal(i.Instance.Status, i.OriginalInstance.Status) {
		i.debug("Updating status")

		if err := i.Client.Status().Update(context.TODO(), i.Instance); err != nil {
			if errors.IsConflict(err) {
				i.Log.Error(err, "Status conflict")
				return reconcile.Result{}, err
			}

			i.error(err, "Error updating pipeline status")
			return reconcile.Result{}, err
		}
	}

	delay := time.Second * time.Duration(i.GetRequeueInterval(i))
	i.debug("RequeueAfter", "delay", delay)
	return reconcile.Result{RequeueAfter: delay}, nil
}

func (i *ReconcileIteration) getValidationInterval() int {
	if i.Instance.Status.InitialSyncInProgress == true {
		return i.config.ValidationInitInterval.Int()
	}

	return i.config.ValidationInterval.Int()
}

func (i *ReconcileIteration) getValidationAttemptsThreshold() int {
	if i.Instance.Status.InitialSyncInProgress == true {
		return i.config.ValidationInitAttemptsThreshold.Int()
	}

	return i.config.ValidationAttemptsThreshold.Int()
}

func (i *ReconcileIteration) getValidationPercentageThreshold() int {
	if i.Instance.Status.InitialSyncInProgress == true {
		return i.config.ValidationInitPercentageThreshold.Int()
	}

	return i.config.ValidationPercentageThreshold.Int()
}
