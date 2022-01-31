#!/bin/bash

pkill -f "kubectl port-forward"

PROJECT_NAME=$1

if [ -z "$PROJECT_NAME" ]; then
  PROJECT_NAME="test"
fi

echo "Using namespace $PROJECT_NAME"

KAFKA_NAME=$(kubectl get kafka -o custom-columns=:metadata.name -n "$PROJECT_NAME" | xargs)
CONNECT_NAME=$(kubectl get kafkaconnect -o custom-columns=:metadata.name -n "$PROJECT_NAME" | xargs)
KAFKA_SVC="svc/$KAFKA_NAME-kafka-bootstrap"
CONNECT_SVC="svc/$CONNECT_NAME-connect-api"
ELASTICSEARCH_SVC="svc/xjoin-elasticsearch-es-http"
HBI_DB_SVC="svc/host-inventory-db"
XJOIN_SVC="svc/xjoin-search"
HBI_SVC="svc/host-inventory-service"
SCHEMA_REGISTRY_SVC="svc/confluent-schema-registry"
APICURIO_SVC="svc/example-apicurioregistry-kafkasql-service"
XJOIN_API_GATEWAY="svc/xjoin-api-gateway-api"
CATS_DB_SVC="svc/cats-db"

kubectl port-forward "$CATS_DB_SVC" 5433:5432 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward "$HBI_DB_SVC" 5432:5432 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward "$CONNECT_SVC" 8083:8083 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward "$ELASTICSEARCH_SVC" 9200:9200 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward "$KAFKA_SVC" 9092:9092 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward "$KAFKA_SVC" 29092:9092 -n "$PROJECT_NAME" >/dev/null 2>&1 &
#kubectl port-forward "$XJOIN_SVC" 4000:4000 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward "$HBI_SVC" 8000:8000 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward "$SCHEMA_REGISTRY_SVC" 9080:8081 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward "$APICURIO_SVC" 1080:8080 -n "$PROJECT_NAME" >/dev/null 2>&1 &
kubectl port-forward "$XJOIN_API_GATEWAY" 8001:8000 -n "$PROJECT_NAME" >/dev/null 2>&1 &

pgrep -fla "kubectl port-forward"
