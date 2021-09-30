package common

import (
	"fmt"
	"github.com/go-errors/errors"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
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
	return fmt.Sprintf("%s", strconv.FormatInt(time.Now().UnixNano(), 10))
}

func (r *Reconciler) Reconcile() (err error) {
	//[REMOVED]
	if r.instance.GetDeletionTimestamp() != nil {
		r.log.Debug("STATE: REMOVED")
		err = r.methods.Removed()
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	//[NEW]
	if r.instance.GetActiveVersion() == "" && r.instance.GetRefreshingVersion() == "" {
		r.log.Debug("STATE: NEW")

		refreshingVersion := r.Version()
		r.instance.SetRefreshingVersion(refreshingVersion)
		r.instance.SetRefreshingVersionIsValid(false)

		err = r.methods.New(refreshingVersion)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	//[INITIAL SYNC]
	if r.instance.GetActiveVersion() == "" &&
		r.instance.GetRefreshingVersionIsValid() == false &&
		r.instance.GetRefreshingVersion() != "" {

		r.log.Debug("STATE: INITIAL_SYNC")
		err = r.methods.InitialSync()
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	//[VALID] ACTIVE IS VALID
	if r.instance.GetActiveVersion() != "" &&
		r.instance.GetActiveVersionIsValid() == true {

		r.log.Debug("STATE: VALID")

		err = r.methods.Valid()
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	//[START REFRESHING] ACTIVE IS INVALID, NOT REFRESHING YET
	if r.instance.GetActiveVersion() != "" &&
		r.instance.GetActiveVersionIsValid() == false &&
		r.instance.GetRefreshingVersion() == "" {

		r.log.Debug("STATE: START REFRESH")
		refreshingVersion := r.Version()
		r.instance.SetRefreshingVersion(refreshingVersion)

		err = r.methods.StartRefreshing(refreshingVersion)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	//[REFRESHING] ACTIVE IS INVALID, REFRESHING IS INVALID
	if r.instance.GetActiveVersion() != "" &&
		r.instance.GetActiveVersionIsValid() == false &&
		r.instance.GetRefreshingVersion() != "" &&
		r.instance.GetRefreshingVersionIsValid() == false {

		r.log.Debug("STATE: REFRESHING")

		err = r.methods.Refreshing()
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}

	//[REFRESH COMPLETE] ACTIVE IS INVALID, REFRESHING IS VALID
	if r.instance.GetActiveVersion() != "" &&
		r.instance.GetActiveVersionIsValid() == false &&
		r.instance.GetRefreshingVersion() != "" &&
		r.instance.GetRefreshingVersionIsValid() == true {

		r.log.Debug("STATE: REFRESH COMPLETE")
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
