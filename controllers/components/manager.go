package components

type Component interface {
	Name() string
	Create() error
	Delete() error
	CheckDeviation() error
}

type ComponentManager struct {
	components map[string]Component
}

func NewComponentManager() *ComponentManager {
	components := make(map[string]Component)
	return &ComponentManager{
		components: components,
	}
}

func (c *ComponentManager) AddComponent(component Component) {
	c.components[component.Name()] = component
}
