package components

import (
	"fmt"
	"github.com/redhatinsights/xjoin-operator/controllers/events"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"

	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
)

type Custodian struct {
	validVersions []string
	name          string
	components    []Component
	kind          string
	events        events.Events
	log           logger.Log
}

func NewCustodian(kind string, name string, validVersions []string, e events.Events, log logger.Log) *Custodian {
	return &Custodian{
		validVersions: validVersions,
		name:          name,
		kind:          kind,
		events:        e,
		log:           log,
	}
}

func (c *Custodian) AddComponent(component Component) {
	component.SetName(c.kind, c.name)
	component.SetEvents(c.events)
	component.SetLogger(c.log)
	c.components = append(c.components, component)
}

// Scrub removes any components not in validVersions.
//
//	When a removal fails it continues scrubbing.
//	Each error is returned when the scrubbing is complete.
func (c *Custodian) Scrub() (allErrors []error) {
	for _, component := range c.components {
		installedVersions, err := component.ListInstalledVersions()
		c.log.Debug(fmt.Sprintf("Installed versions for component %T during scrub", component), "installedVersion", installedVersions)
		if err != nil {
			err = fmt.Errorf("%w; Unable to list versions for component while scrubbing: %T, %s", err, component, component.Name())
			allErrors = append(allErrors, errors.Wrap(err, 0))
			continue
		}

		for _, installedVersion := range installedVersions {
			if !utils.ContainsString(c.validVersions, installedVersion) {
				component.SetVersion(installedVersion)
				err = component.Delete()
				c.log.Debug(fmt.Sprintf("Deleted component %T during scrub", component), "version", installedVersion)
				if err != nil {
					err = fmt.Errorf("%w; Unable to delete component while scrubbing: %s", err, component.Name())
					allErrors = append(allErrors, errors.Wrap(err, 0))
				}
			}
		}
	}

	return allErrors
}
