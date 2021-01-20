package config

import (
	"errors"
	"fmt"
	logger "github.com/redhatinsights/xjoin-operator/controllers/log"
	"reflect"
)

var log = logger.NewLogger("parameter")

type Parameter struct {
	SpecKey      string
	Secret       string
	SecretKey    string
	ConfigMapKey string
	DefaultValue interface{}
	value        interface{}
	Type         reflect.Kind
}

func (p *Parameter) String() string {
	if p.value == nil {
		return p.DefaultValue.(string)
	} else {
		return p.value.(string)
	}
}

func (p *Parameter) Int() int {
	if p.value == nil {
		return p.DefaultValue.(int)
	} else {
		return p.value.(int)
	}
}

func (p *Parameter) Bool() bool {
	if p.value == nil {
		return p.DefaultValue.(bool)
	} else {
		return p.value.(bool)
	}
}

func (p *Parameter) Value() interface{} {
	if p.value == nil {
		return p.DefaultValue
	} else {
		return p.value
	}
}

func (p *Parameter) SetValue(value interface{}) error {
	log.Trace(fmt.Sprintf(
		"Setting value to: %s for parameter: \nCM key: %s \nSpec key: %s \nSecret key: %s%s",
		value, p.ConfigMapKey, p.SpecKey, p.Secret, p.SecretKey))

	if value != nil {
		t := reflect.TypeOf(value).Kind()

		if t == reflect.Ptr {
			if reflect.ValueOf(value).IsNil() {
				return nil
			} else {
				t = reflect.ValueOf(value).Elem().Type().Kind()
				value = reflect.ValueOf(value).Elem().Interface()
			}
		}

		if t != p.Type {
			return errors.New(fmt.Sprintf(
				"Value must be of type %s.\nValue type: %s \nCM key: %s \nSpec key: %s \nSecret key: %s.%s",
				p.Type.String(), t.String(), p.ConfigMapKey, p.SpecKey, p.Secret, p.SecretKey))
		} else {
			p.value = value
		}
	}

	return nil
}
