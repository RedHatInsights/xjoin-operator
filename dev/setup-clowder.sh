#!/bin/bash

function print_start_message() {
  echo -e "\n********************************************************************************"
  echo -e "* $1"
  echo -e "********************************************************************************\n"
}

# kube_setup.sh
print_start_message "Running kube-setup.sh"
mkdir /tmp/kubesetup
#curl https://raw.githubusercontent.com/RedHatInsights/clowder/master/build/kube_setup.sh -o /tmp/kubesetup/kube_setup.sh && chmod +x /tmp/kubesetup/kube_setup.sh
#/tmp/kubesetup/kube_setup.sh
/home/chris/dev/projects/active/gopath/src/clowder/build/kube_setup.sh
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
kubectl patch KafkaConnect connect --patch '{"spec": {"config": {"connector.client.config.override.policy": "All"}}}' --type=merge -n test

sleep 5

# inventory resources
print_start_message "Setting up host-inventory"
bonfire deploy host-inventory -n test

dev/forward-ports-clowder.sh test
HBI_USER=$(kubectl -n test get secret/host-inventory-db -o custom-columns=:data.username | base64 -d)
HBI_NAME=$(kubectl -n test get secret/host-inventory-db -o custom-columns=:data.name | base64 -d)
psql -U "$HBI_USER" -h inventory-db -p 5432 -d "$HBI_NAME" -c "CREATE USER insights WITH PASSWORD 'insights' SUPERUSER;"
psql -U "$HBI_USER" -h inventory-db -p 5432 -d "$HBI_NAME" -c "ALTER ROLE insights REPLICATION LOGIN;"
psql -U "$HBI_USER" -h inventory-db -p 5432 -d "$HBI_NAME" -c "CREATE PUBLICATION dbz_publication FOR TABLE hosts;"
psql -U "$HBI_USER" -h inventory-db -p 5432 -d "$HBI_NAME" -c "CREATE DATABASE test WITH TEMPLATE '$HBI_NAME';"

# elasticsearch
print_start_message "Setting up elasticsearch"
dev/setup.sh --elasticsearch --clowder --project test

# operator CRDs, configmap, XJoinPipeline
print_start_message "Setting up XJoin operator"
dev/setup.sh --xjoin-operator --clowder --dev --project test
kubectl patch KafkaConnect connect --patch '{"spec": {"config": {"connector.client.config.override.policy": "All"}}}' --type=merge -n test

# xjoin-search
print_start_message "Setting up xjoin-search"
bonfire deploy xjoin-search -n test
