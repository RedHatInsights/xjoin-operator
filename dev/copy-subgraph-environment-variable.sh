#!/bin/bash
kubectl get -o yaml pods --selector="xjoin.index=$1" | yq eval ".items[0].spec.containers[].env | .[] | select(.name == \"$2\") | .value" -
