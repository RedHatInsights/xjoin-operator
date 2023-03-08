#!/bin/bash
# this scripts helps with quickly getting credentials for accessing k8s resources.

PROJECT_NAME=$1

if [ -z "$PROJECT_NAME" ]; then
  echo "openshift project or Kubernetes namespace required as input argument."
  exit 1
fi

echo "Using namespace $PROJECT_NAME"

# host inventory
kubectl -n $PROJECT_NAME get secret/host-inventory-db -o custom-columns=:data.username | base64 -d | xargs echo username:  
kubectl -n $PROJECT_NAME get secret/host-inventory-db -o custom-columns=:data.password | base64 -d | xargs echo password:  
kubectl -n $PROJECT_NAME get secret/host-inventory-db -o custom-columns=:data.name | base64 -d | xargs echo db-name:  
kubectl -n $PROJECT_NAME get secret/host-inventory-db -o custom-columns=:data.hostname | base64 -d | xargs echo hostname:  

# elasticsearch
kubectl -n $PROJECT_NAME get secret/xjoin-elasticsearch-es-elastic-user -o custom-columns=:data.elastic | base64 -d | xargs echo elastic-user: 

echo "Done!"
