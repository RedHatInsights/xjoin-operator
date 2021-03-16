#!/bin/bash
NAMESPACE=xjoin-operator-olm

oc create ns ${NAMESPACE}
oc project ${NAMESPACE}
oc apply -f config/samples/hbisecret.yaml
oc process -f ./deploy/operator.yml -p TARGET_NAMESPACE=${NAMESPACE} -o yaml | oc apply -f -
