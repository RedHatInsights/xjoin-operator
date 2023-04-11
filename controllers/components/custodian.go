package components

import (
	"fmt"
	"github.com/go-errors/errors"
	"github.com/redhatinsights/xjoin-go-lib/pkg/utils"
)

type Custodian struct {
	validVersions []string
	name          string
	components    []Component
}

func NewCustodian(name string, validVersions []string) *Custodian {
	return &Custodian{
		validVersions: validVersions,
		name:          name,
	}
}

func (c *Custodian) AddComponent(component Component) {
	component.SetName(c.name)
	c.components = append(c.components, component)
}

// Scrub removes any components not in validVersions.
//
//	When a removal fails it continues scrubbing.
//	Each error is returned when the scrubbing is complete.
func (c *Custodian) Scrub() (allErrors []error) {
	for _, component := range c.components {
		installedVersions, err := component.ListInstalledVersions()
		if err != nil {
			err = fmt.Errorf("%w; Unable to list versions for component while scrubbing: %s", err, component.Name())
			allErrors = append(allErrors, errors.Wrap(err, 0))
			continue
		}

		for _, installedVersion := range installedVersions {
			if !utils.ContainsString(c.validVersions, installedVersion) {
				component.SetVersion(installedVersion)
				err = component.Delete()
				if err != nil {
					err = fmt.Errorf("%w; Unable to delete component while scrubbing: %s", err, component.Name())
					allErrors = append(allErrors, errors.Wrap(err, 0))
				}
			}
		}
	}

	return allErrors
}
