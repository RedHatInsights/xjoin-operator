package index

import (
	"github.com/redhatinsights/xjoin-operator/controllers/common"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const xjoinindexvalidatorFinalizer = "finalizer.xjoin.indexvalidator.cloud.redhat.com"

type XJoinIndexValidatorIteration struct {
	common.Iteration
	Parameters parameters.IndexParameters
}

func (i *XJoinIndexValidatorIteration) Finalize() (err error) {
	i.Log.Info("Starting finalizer")
	controllerutil.RemoveFinalizer(i.Instance, xjoinindexvalidatorFinalizer)

	ctx, cancel := utils.DefaultContext()
	defer cancel()
	err = i.Client.Update(ctx, i.Instance)
	if err != nil {
		return
	}

	i.Log.Info("Successfully finalized")
	return nil
}

func (i *XJoinIndexValidatorIteration) Validate() (err error) {
	return
}
