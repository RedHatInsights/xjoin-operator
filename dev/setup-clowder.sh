#!/bin/bash

function print_start_message() {
  echo -e "\n********************************************************************************"
  echo -e "* $1"
  echo -e "********************************************************************************\n"
}

# kube_setup.sh
print_start_message "Running kube-setup.sh"
mkdir /tmp/kubesetup
curl https://raw.githubusercontent.com/RedHatInsights/clowder/master/build/kube_setup.sh -o /tmp/kube_setup.sh && chmod +x /tmp/kube_setup.sh
/tmp/kube_setup.sh
rm -r /tmp/kubesetup

# clowder CRDs
print_start_message "Installing Clowder CRDs"
kubectl apply -f https://github.com/RedHatInsights/clowder/releases/download/0.15.0/clowder-manifest-0.15.0.yaml --validate=false

# project and secrets
print_start_message "Setting up pull secrets"
dev/setup.sh --clowder --secret --project test

# bonfire environment (kafka, connect, etc.)
print_start_message "Setting up bonfire environment"
bonfire deploy-env -n test
kubectl patch env env-test --patch '{"spec": {"providers": {"pullSecrets": [{"name": "xjoin-pull-secret", "namespace": "test"}]}}}' --type=merge
kubectl patch KafkaConnect connect --patch '{"spec": {"config": {"connector.client.config.override.policy": "All"}}}' --type=merge

sleep 5

# inventory resources
print_start_message "Setting up host-inventory"

# there is a bug in the bonfire inventory deployment which prevents the image from being pulled.
# So pull the image before starting the bonfire deployment
kubectl run -n test inventory-image-pull --image=quay.io/cloudservices/insights-inventory:latest --command -- /bin/sh -c "sleep 30"
kubectl wait pods/inventory-image-pull --for=condition=Ready --timeout=150s -n test
kubectl delete pods/inventory-image-pull -n test

bonfire deploy host-inventory -i quay.io/cloudservices/insights-inventory=latest -n test

dev/forward-ports-clowder.sh test
HBI_USER=$(kubectl -n test get secret/host-inventory-db -o custom-columns=:data.username | base64 -d)
HBI_NAME=$(kubectl -n test get secret/host-inventory-db -o custom-columns=:data.name | base64 -d)
psql -U "$HBI_USER" -h inventory-db -p 5432 -d "$HBI_NAME" -c "ALTER SYSTEM SET wal_level = logical;"

# elasticsearch
print_start_message "Setting up elasticsearch"
dev/setup.sh --elasticsearch --clowder --project test

# operator CRDs, configmap, XJoinPipeline
print_start_message "Setting up XJoin operator"
dev/setup.sh --xjoin-operator --clowder --project test