#!/bin/bash

pkill -f "kubectl port-forward"

kubectl port-forward svc/inventory-db 5432:5432 -n xjoin-operator-project >/dev/null 2>&1 &
kubectl port-forward services/xjoin-kafka-connect-strimzi-connect-api 8083:8083 -n xjoin-operator-project >/dev/null 2>&1 &
kubectl port-forward svc/xjoin-elasticsearch-es-http 9200:9200 -n xjoin-operator-project >/dev/null 2>&1 &
kubectl port-forward svc/xjoin-kafka-cluster-kafka-bootstrap 9092:9092 -n xjoin-operator-project >/dev/null 2>&1 &

pgrep -fla "kubectl port-forward"
