package components

import (
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

//Scrub removes any components not in validVersions
func (c *Custodian) Scrub() error {
	for _, component := range c.components {
		installedVersions, err := component.ListInstalledVersions()
		if err != nil {
			return errors.Wrap(err, 0)
		}

		for _, installedVersion := range installedVersions {
			if !utils.ContainsString(c.validVersions, installedVersion) {
				component.SetVersion(installedVersion)
				err = component.Delete()
				if err != nil {
					return errors.Wrap(err, 0)
				}
			}
		}
	}

	return nil
}
