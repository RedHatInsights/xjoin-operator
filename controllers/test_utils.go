package controllers

import (
	"github.com/go-errors/errors"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

var K8sGetTimeout = 3 * time.Second
var K8sGetInterval = 100 * time.Millisecond

var testLogger = logf.Log.WithName("test")

//this is used to output stack traces when an error occurs
func checkError(err error) {
	if err != nil {
		testLogger.Error(errors.Wrap(err, 0), "test failure")
	}
	Expect(err).ToNot(HaveOccurred())
}
