#!/bin/bash

pkill -f "oc port-forward"

oc port-forward svc/inventory-db 5432:5432 -n xjoin-operator-project &
oc port-forward services/xjoin-kafka-connect-strimzi-connect-api 8083:8083 -n xjoin-operator-project &
oc port-forward svc/xjoin-elasticsearch-es-http 9200:9200 -n xjoin-operator-project &
