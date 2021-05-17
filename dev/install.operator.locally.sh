#!/bin/bash
NAMESPACE=xjoin-operator-olm

kubectl create ns ${NAMESPACE}
kubectl project ${NAMESPACE}
kubectl apply -f config/samples/hbisecret.yaml
kubectl process -f ./deploy/operator.yml -p TARGET_NAMESPACE=${NAMESPACE} -o yaml | kubectl apply -f -
