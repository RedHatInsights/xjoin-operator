#!/bin/bash

./dev/forward-ports-clowder.sh

#kubectl port-forward svc/ksql-server 8088:8088 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward svc/confluent-schema-registry 8081:8081 -n "$PROJECT_NAME" >/dev/null 2>&1 &