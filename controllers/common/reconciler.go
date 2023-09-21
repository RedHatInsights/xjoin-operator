package common

import (
	"github.com/go-errors/errors"
	validation "github.com/redhatinsights/xjoin-go-lib/pkg/validation"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"time"
)

type ReconcilerMethods interface {
	Removed() error
	New(string) error
	InitialSync() error
	Valid() error
	StartRefreshing(string) error
	Refreshing() error
	RefreshComplete() error
	RefreshFailed() error
	Scrub() []error
	SetLogger(logger.Log)
	SetIsTest(bool)
}

type Reconciler struct {
	methods  ReconcilerMethods
	instance XJoinObject
	log      logger.Log
	events   events.Events
	isTest   bool
}

func NewReconciler(methods ReconcilerMethods, instance XJoinObject, log logger.Log, e events.Events, isTest bool) *Reconciler {
	methods.SetLogger(log)
	methods.SetIsTest(isTest)

	return &Reconciler{
		methods:  methods,
		instance: instance,
		log:      log,
		events:   e,
		isTest:   isTest,
	}
}

func (r *Reconciler) Version() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func (r *Reconciler) DoRefresh() (err error) {
	refreshingVersion := r.Version()
	r.instance.SetRefreshingVersion(refreshingVersion)

	err = r.methods.StartRefreshing(refreshingVersion)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return
}

const (
	START_REFRESH    string = "START_REFRESH"
	REFRESHING       string = "REFRESHING"
	NEW              string = "NEW"
	REMOVED          string = "REMOVED"
	INITIAL_SYNC     string = "INITIAL_SYNC"
	VALID            string = "VALID"
	REFRESH_COMPLETE string = "REFRESH_COMPLETE"
	REFRESH_FAILED   string = "REFRESH_FAILED"
)

const (
	ValidConditionType         string = "Valid"
	NewReason                  string = "New"
	UnknownReason              string = "Unknown"
	ValidationSucceededReason  string = "ValidationSucceeded"
	ValidationFailedReason     string = "ValidationFailed"
	ValidationInProgressReason string = "ValidationInProgress"
	RefreshFailedReason        string = "RefreshFailed"
	RefreshingReason           string = "RefreshInProgress"
	RefreshCompleteReason      string = "RefreshComplete"
	RemovedReason              string = "Removed"
	DeviationReason            string = "DeviationFound"
)

func (r *Reconciler) getState(specHash string) string {
	if r.instance.GetDeletionTimestamp() != nil {
		return REMOVED
	} else if (r.instance.GetActiveVersion() != "" &&
		r.instance.GetActiveVersionState() != validation.ValidationValid &&
		r.instance.GetRefreshingVersion() == "") ||
		(r.instance.GetSpecHash() != "" && r.instance.GetSpecHash() != specHash) {
		return START_REFRESH
	} else if r.instance.GetActiveVersion() == "" && r.instance.GetRefreshingVersion() == "" {
		return NEW
	} else if r.instance.GetActiveVersion() == "" &&
		r.instance.GetRefreshingVersionState() == validation.ValidationNew &&
		r.instance.GetRefreshingVersion() != "" {
		return INITIAL_SYNC
	} else if r.instance.GetRefreshingVersion() != "" &&
		r.instance.GetRefreshingVersionState() == validation.ValidationValid {
		return REFRESH_COMPLETE
	} else if r.instance.GetActiveVersion() != "" &&
		r.instance.GetActiveVersionState() == validation.ValidationInvalid &&
		r.instance.GetRefreshingVersion() != "" &&
		r.instance.GetRefreshingVersionState() == validation.ValidationNew {
		return REFRESHING
	} else if r.instance.GetRefreshingVersion() != "" &&
		r.instance.GetRefreshingVersionState() == validation.ValidationInvalid {
		return REFRESH_FAILED
	} else if r.instance.GetActiveVersion() != "" &&
		r.instance.GetActiveVersionState() == validation.ValidationValid {
		return VALID
	} else {
		return ""
	}
}

func (r *Reconciler) Reconcile(forceRefresh bool) (err error) {
	//Scrub orphaned resources
	errs := r.methods.Scrub()
	if len(errs) > 0 {
		for _, e := range errs {
			r.log.Error(e, e.Error())
		}
	}

	specHash, err := k8sUtils.SpecHash(r.instance.GetSpec())
	if err != nil {
		return errors.Wrap(err, 0)
	}

	state := r.getState(specHash)
	if state != REFRESHING && forceRefresh {
		state = START_REFRESH
	}
	r.log.Debug("Reconciler state", "state", state)

	switch state {
	case REMOVED:
		r.instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionFalse,
			Reason: RemovedReason,
		})

		err = r.methods.Removed()
		if err != nil {
			return errors.Wrap(err, 0)
		}
	case START_REFRESH:
		// active is invalid, not refreshing yet
		// or spec hash mismatch
		// or force_refresh is true
		r.instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionFalse,
			Reason: RefreshingReason,
		})

		refreshingVersion := r.Version()
		r.instance.SetRefreshingVersion(refreshingVersion)
		r.instance.SetRefreshingVersionState(validation.ValidationNew)

		err = r.methods.StartRefreshing(refreshingVersion)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	case NEW:
		r.instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionFalse,
			Reason: NewReason,
		})

		refreshingVersion := r.Version()
		r.instance.SetRefreshingVersion(refreshingVersion)
		r.instance.SetRefreshingVersionState(validation.ValidationNew)

		err = r.methods.New(refreshingVersion)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	case INITIAL_SYNC:
		r.instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionFalse,
			Reason: ValidationInProgressReason,
		})

		err = r.methods.InitialSync()
		if err != nil {
			return errors.Wrap(err, 0)
		}
	case VALID:
		r.instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionTrue,
			Reason: ValidationSucceededReason,
		})

		err = r.methods.Valid()
		if err != nil {
			return errors.Wrap(err, 0)
		}

		r.instance.SetRefreshingVersion("")
		r.instance.SetRefreshingVersionState("")
	case REFRESHING:
		// active is invalid, refreshing is invalid
		r.instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionFalse,
			Reason: RefreshingReason,
		})

		err = r.methods.Refreshing()
		if err != nil {
			return errors.Wrap(err, 0)
		}
	case REFRESH_COMPLETE:
		r.instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionTrue,
			Reason: RefreshCompleteReason,
		})

		err = r.methods.RefreshComplete()
		if err != nil {
			return errors.Wrap(err, 0)
		}

		r.instance.SetActiveVersion(r.instance.GetRefreshingVersion())
		r.instance.SetActiveVersionState(validation.ValidationValid)
		r.instance.SetRefreshingVersion("")
		r.instance.SetRefreshingVersionState("")
	case REFRESH_FAILED:
		r.instance.SetCondition(metav1.Condition{
			Type:   ValidConditionType,
			Status: metav1.ConditionFalse,
			Reason: RefreshFailedReason,
		})

		err = r.methods.RefreshFailed()
		if err != nil {
			return errors.Wrap(err, 0)
		}

		r.instance.SetRefreshingVersion("")
		r.instance.SetRefreshingVersionState("")
	}

	return nil
}
