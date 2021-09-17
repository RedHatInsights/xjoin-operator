package components

import "github.com/go-errors/errors"

type Component interface {
	Name() string
	Create() error
	Delete() error
	CheckDeviation() error
	Exists() (bool, error)
}

type ComponentManager struct {
	components []Component
	version    string
}

func NewComponentManager(version string) ComponentManager {
	return ComponentManager{
		version: version,
	}
}

func (c *ComponentManager) AddComponent(component Component) {
	c.components = append(c.components, component)
}

//CreateAll creates all components. No-op if the components are already created.
func (c ComponentManager) CreateAll() error {
	for _, component := range c.components {
		componentExists, err := component.Exists()
		if err != nil {
			return errors.Wrap(err, 0)
		}
		if !componentExists {
			err = component.Create()
			if err != nil {
				return errors.Wrap(err, 0)
			}
		}
	}
	return nil
}

//DeleteAll deletes all components. No-op if the components are already deleted.
func (c ComponentManager) DeleteAll() error {
	for _, component := range c.components {
		componentExists, err := component.Exists()
		if err != nil {
			return err
		}
		if componentExists {
			err = component.Delete()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
