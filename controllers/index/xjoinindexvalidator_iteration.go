package index

import (
	xjoin "github.com/redhatinsights/xjoin-operator/api/v1alpha1"
	xjoinlogger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"github.com/redhatinsights/xjoin-operator/controllers/parameters"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type XJoinIndexValidatorIteration struct {
	Instance         *xjoin.XJoinIndexValidator
	Client           client.Client
	Log              xjoinlogger.Log
	Parameters       parameters.IndexParameters
	OriginalInstance xjoin.XJoinIndexValidator
}
