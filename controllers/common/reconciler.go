package common

import (
	"github.com/go-errors/errors"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	k8sUtils "github.com/redhatinsights/xjoin-operator/controllers/utils"
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
	Scrub() error
}

type Reconciler struct {
	methods  ReconcilerMethods
	instance XJoinObject
	log      logger.Log
}

func NewReconciler(methods ReconcilerMethods, instance XJoinObject, log logger.Log) *Reconciler {
	return &Reconciler{
		methods:  methods,
		instance: instance,
		log:      log,
	}
}

func (r *Reconciler) Version() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func (r *Reconciler) DoRefresh() (err error) {
	r.log.Info("STATE: START REFRESH")

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
)

func (r *Reconciler) getState(specHash string) string {
	if r.instance.GetDeletionTimestamp() != nil {
		return REMOVED
	} else if (r.instance.GetActiveVersion() != "" &&
		!r.instance.GetActiveVersionIsValid() &&
		r.instance.GetRefreshingVersion() == "") ||
		(r.instance.GetSpecHash() != "" && r.instance.GetSpecHash() != specHash) {
		return START_REFRESH
	} else if r.instance.GetActiveVersion() == "" && r.instance.GetRefreshingVersion() == "" {
		return NEW
	} else if r.instance.GetActiveVersion() == "" &&
		!r.instance.GetRefreshingVersionIsValid() &&
		r.instance.GetRefreshingVersion() != "" {
		return INITIAL_SYNC
	} else if r.instance.GetRefreshingVersion() != "" &&
		r.instance.GetRefreshingVersionIsValid() {
		return REFRESH_COMPLETE
	} else if r.instance.GetActiveVersion() != "" &&
		!r.instance.GetActiveVersionIsValid() &&
		r.instance.GetRefreshingVersion() != "" &&
		!r.instance.GetRefreshingVersionIsValid() {
		return REFRESHING
	} else if r.instance.GetActiveVersion() != "" &&
		r.instance.GetActiveVersionIsValid() {
		return VALID
	} else {
		return ""
	}
}

func (r *Reconciler) Reconcile(forceRefresh bool) (err error) {
	specHash, err := k8sUtils.SpecHash(r.instance.GetSpec())
	if err != nil {
		return errors.Wrap(err, 0)
	}

	state := r.getState(specHash)
	if state != REFRESHING && forceRefresh {
		state = START_REFRESH
	}

	switch state {
	case REMOVED:
		r.log.Info("STATE: REMOVED")

		err = r.methods.Removed()
		if err != nil {
			return errors.Wrap(err, 0)
		}
	case START_REFRESH:
		// active is invalid, not refreshing yet
		// or spec hash mismatch
		// or force_refresh is true
		r.log.Info("STATE: START REFRESH")

		refreshingVersion := r.Version()
		r.instance.SetRefreshingVersion(refreshingVersion)

		err = r.methods.StartRefreshing(refreshingVersion)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	case NEW:
		r.log.Info("STATE: NEW")

		refreshingVersion := r.Version()
		r.instance.SetRefreshingVersion(refreshingVersion)
		r.instance.SetRefreshingVersionIsValid(false)

		err = r.methods.New(refreshingVersion)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	case INITIAL_SYNC:
		r.log.Info("STATE: INITIAL_SYNC")
		err = r.methods.InitialSync()
		if err != nil {
			return errors.Wrap(err, 0)
		}
	case VALID:
		r.log.Info("STATE: VALID")

		err = r.methods.Valid()
		if err != nil {
			return errors.Wrap(err, 0)
		}
	case REFRESHING:
		// active is invalid, refreshing is invalid
		r.log.Info("STATE: REFRESHING")

		err = r.methods.Refreshing()
		if err != nil {
			return errors.Wrap(err, 0)
		}
	case REFRESH_COMPLETE:
		r.log.Info("STATE: REFRESH COMPLETE")
		err = r.methods.RefreshComplete()
		if err != nil {
			return errors.Wrap(err, 0)
		}

		r.instance.SetActiveVersion(r.instance.GetRefreshingVersion())
		r.instance.SetActiveVersionIsValid(true)
		r.instance.SetRefreshingVersion("")
		r.instance.SetRefreshingVersionIsValid(false)
	}

	//Scrub orphaned resources
	err = r.methods.Scrub()
	if err != nil {
		return errors.Wrap(err, 0)
	}

	return nil
}
