#!/bin/bash
NAMESPACE=xjoin-operator-olm

kubectl create ns ${NAMESPACE}
kubectl project ${NAMESPACE}
kubectl apply -f config/samples/hbisecret.yaml
oc process -f ./deploy/operator.yml -p CONNECT_CLUSTER=connect KAFKA_CLUSTER=kafka KAFKA_CLUSTER_NAMESPACE=${NAMESPACE} CONNECT_CLUSTER_NAMESPACE=${NAMESPACE} ELASTICSEARCH_CONNECTOR_TASKS_MAX=5 -o yaml | kubectl apply -f -