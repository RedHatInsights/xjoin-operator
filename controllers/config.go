package controllers

import (
	"fmt"
	"github.com/redhatinsights/xjoin-operator/controllers/config"
	"github.com/redhatinsights/xjoin-operator/controllers/utils"
	"k8s.io/apimachinery/pkg/api/errors"
)

const configMapName = "xjoin"

func (i *ReconcileIteration) parseConfig() error {
	cyndiConfig, err := utils.FetchConfigMap(i.Client, i.Instance.Namespace, configMapName)

	if err != nil {
		if errors.IsNotFound(err) {
			cyndiConfig = nil
		} else {
			return err
		}
	}

	i.config, err = config.BuildXJoinConfig(i.Instance, cyndiConfig)

	if err != nil {
		return fmt.Errorf("Error parsing %s configmap in %s: %w", configMapName, i.Instance.Namespace, err)
	}

	return err
}
