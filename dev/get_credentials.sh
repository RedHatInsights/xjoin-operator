#!/bin/bash
# this scripts helps with quickly getting credentials for accessing k8s resources.

PROJECT_NAME=$1

if [ -z "$PROJECT_NAME" ]; then
  echo "openshift project or Kubernetes namespace required as input argument."
  exit 1
fi

echo -e "Using namespace $PROJECT_NAME\n"

# host inventory
HBI_DATABASE_SECRET_NAME="host-inventory-db"
HAS_INVENTORY_SECRET=$(kubectl get secrets -o json -n $PROJECT_NAME | jq ".items[] | select(.metadata.name==\"$HBI_DATABASE_SECRET_NAME\")")
if [ -n "$HAS_INVENTORY_SECRET" ]; then
  HBI_USER=$(kubectl -n "$PROJECT_NAME" get secret/"$HBI_DATABASE_SECRET_NAME" -o custom-columns=:data.username | base64 -d)
  HBI_PASSWORD=$(kubectl -n "$PROJECT_NAME" get secret/"$HBI_DATABASE_SECRET_NAME" -o custom-columns=:data.password | base64 -d)
  HBI_NAME=$(kubectl -n "$PROJECT_NAME" get secret/"$HBI_DATABASE_SECRET_NAME" -o custom-columns=:data.name | base64 -d)
  HBI_HOSTNAME=$(kubectl -n "$PROJECT_NAME" get secret/"$HBI_DATABASE_SECRET_NAME" -o custom-columns=:data.hostname | base64 -d)
  echo "HBI_USER: $HBI_USER"
  echo "HBI_PASSWORD: $HBI_PASSWORD"
  echo "HBI_NAME: $HBI_NAME"
  echo "HBI_HOSTNAME: $HBI_HOSTNAME"
  echo -e ""
  export HBI_USER
  export HBI_PASSWORD
  export HBI_NAME
  export HBI_HOSTNAME
else
  echo "$HBI_DATABASE_SECRET_NAME secret not found"
fi

# elasticsearch
ELASTICSEARCH_SECRET_NAME="xjoin-elasticsearch-es-elastic-user"
HAS_ELASTICSEARCH_SECRET=$(kubectl get secrets -o json -n $PROJECT_NAME | jq ".items[] | select(.metadata.name==\"$ELASTICSEARCH_SECRET_NAME\")")
if [ -n "$HAS_ELASTICSEARCH_SECRET" ]; then
  ES_USERNAME=elastic
  ES_PASSWORD=$(kubectl -n "$PROJECT_NAME" get secret/"$ELASTICSEARCH_SECRET_NAME" -o custom-columns=:data.elastic | base64 -d)
  echo "ES_USERNAME: $ES_USERNAME"
  echo "ES_PASSWORD: $ES_PASSWORD"
  echo -e ""
  export ES_USERNAME
  export ES_PASSWORD
fi

# Payload tracker
PAYLOAD_TRACKER_DATABASE_SECRET_NAME="payload-tracker-db"
HAS_INVENTORY_SECRET=$(kubectl get secrets -o json -n $PROJECT_NAME | jq ".items[] | select(.metadata.name==\"$PAYLOAD_TRACKER_DATABASE_SECRET_NAME\")")
if [ -n "$HAS_INVENTORY_SECRET" ]; then
  PAYLOAD_TRACKER_USER=$(kubectl -n "$PROJECT_NAME" get secret/"$PAYLOAD_TRACKER_DATABASE_SECRET_NAME" -o custom-columns=:data.username | base64 -d)
  PAYLOAD_TRACKER_PASSWORD=$(kubectl -n "$PROJECT_NAME" get secret/"$PAYLOAD_TRACKER_DATABASE_SECRET_NAME" -o custom-columns=:data.password | base64 -d)
  PAYLOAD_TRACKER_NAME=$(kubectl -n "$PROJECT_NAME" get secret/"$PAYLOAD_TRACKER_DATABASE_SECRET_NAME" -o custom-columns=:data.name | base64 -d)
  PAYLOAD_TRACKER_HOSTNAME=$(kubectl -n "$PROJECT_NAME" get secret/"$PAYLOAD_TRACKER_DATABASE_SECRET_NAME" -o custom-columns=:data.hostname | base64 -d)
  echo "PAYLOAD_TRACKER_USER: $PAYLOAD_TRACKER_USER"
  echo "PAYLOAD_TRACKER_PASSWORD: $PAYLOAD_TRACKER_PASSWORD"
  echo "PAYLOAD_TRACKER_NAME: $PAYLOAD_TRACKER_NAME"
  echo "PAYLOAD_TRACKER_HOSTNAME: $PAYLOAD_TRACKER_HOSTNAME"
  echo -e ""
  export PAYLOAD_TRACKER_USER
  export PAYLOAD_TRACKER_PASSWORD
  export PAYLOAD_TRACKER_NAME
  export PAYLOAD_TRACKER_HOSTNAME
else
  echo "$PAYLOAD_TRACKER_DATABASE_SECRET_NAME secret not found"
fi

# advisor
ADVISOR_SECRET_NAME="advisor-backend-db"
HAS_ADVISOR_SECRET=$(kubectl get secrets -o json  -n $PROJECT_NAME | jq ".items[] | select(.metadata.name==\"$ADVISOR_SECRET_NAME\")")
if [ -n "$HAS_ADVISOR_SECRET" ]; then
  ADVISOR_USER=$(kubectl -n "$PROJECT_NAME" get secret/"$ADVISOR_SECRET_NAME" -o custom-columns=:data.username | base64 -d)
  ADVISOR_PASSWORD=$(kubectl -n "$PROJECT_NAME" get secret/"$ADVISOR_SECRET_NAME" -o custom-columns=:data.password | base64 -d)
  ADVISOR_NAME=$(kubectl -n "$PROJECT_NAME" get secret/"$ADVISOR_SECRET_NAME" -o custom-columns=:data.name | base64 -d)
  ADVISOR_HOSTNAME=$(kubectl -n "$PROJECT_NAME" get secret/"$ADVISOR_SECRET_NAME" -o custom-columns=:data.hostname | base64 -d)
  echo "ADVISOR_USER: $ADVISOR_USER"
  echo "ADVISOR_PASSWORD: $ADVISOR_PASSWORD"
  echo "ADVISOR_NAME: $ADVISOR_NAME"
  echo "ADVISOR_HOSTNAME: $ADVISOR_HOSTNAME"
  echo -e ""
  export ADVISOR_USER
  export ADVISOR_PASSWORD
  export ADVISOR_NAME
  export ADVISOR_HOSTNAME
fi

# compliance
COMPLIANCE_SECRET_NAME="compliance-db"
HAS_COMPLIANCE_SECRET=$(kubectl get secrets -o json  -n $PROJECT_NAME | jq ".items[] | select(.metadata.name==\"$COMPLIANCE_SECRET_NAME\")")
if [ -n "$HAS_COMPLIANCE_SECRET" ]; then
  COMPLIANCE_USER=$(kubectl -n "$PROJECT_NAME" get secret/"$COMPLIANCE_SECRET_NAME" -o custom-columns=:data.username | base64 -d)
  COMPLIANCE_PASSWORD=$(kubectl -n "$PROJECT_NAME" get secret/"$COMPLIANCE_SECRET_NAME" -o custom-columns=:data.password | base64 -d)
  COMPLIANCE_NAME=$(kubectl -n "$PROJECT_NAME" get secret/"$COMPLIANCE_SECRET_NAME" -o custom-columns=:data.name | base64 -d)
  COMPLIANCE_HOSTNAME=$(kubectl -n "$PROJECT_NAME" get secret/"$COMPLIANCE_SECRET_NAME" -o custom-columns=:data.hostname | base64 -d)
  echo "COMPLIANCE_USER: $COMPLIANCE_USER"
  echo "COMPLIANCE_PASSWORD: $COMPLIANCE_PASSWORD"
  echo "COMPLIANCE_NAME: $COMPLIANCE_NAME"
  echo "COMPLIANCE_HOSTNAME: $COMPLIANCE_HOSTNAME"
  echo -e ""
  export COMPLIANCE_USER
  export COMPLIANCE_PASSWORD
  export COMPLIANCE_NAME
  export COMPLIANCE_HOSTNAME
fi

echo "Done!"
