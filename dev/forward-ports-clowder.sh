#!/bin/bash

pkill -f "kubectl port-forward"

PROJECT_NAME=$1

if [ -z "$PROJECT_NAME" ]; then
  PROJECT_NAME="test"
fi

echo "Using namespace $PROJECT_NAME"

kubectl port-forward svc/host-inventory-db 5432:5432 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward services/connect-connect-api 8083:8083 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward svc/xjoin-elasticsearch-es-http 9200:9200 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward svc/kafka-kafka-bootstrap 9092:9092 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward svc/kafka-kafka-bootstrap 29092:9092 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward svc/xjoin-search 4000:4000 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward svc/host-inventory-service 8000:8000 -n "$PROJECT_NAME" >/dev/null 2>&1 &


pgrep -fla "kubectl port-forward"
