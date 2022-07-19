#!/bin/bash
kubectl get -o yaml deployments --selector="xjoin.index=$1" | yq eval ".items[0].spec.template.spec.containers[].env | .[] | select(.name == \"$2\") | .value" -
