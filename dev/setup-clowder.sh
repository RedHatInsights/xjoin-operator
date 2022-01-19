#!/bin/bash

function print_start_message() {
  echo -e "\n********************************************************************************"
  echo -e "* $1"
  echo -e "********************************************************************************\n"
}

function wait_for_pod_to_be_created() {
  POD_NAME=$1
  # shellcheck disable=SC2034
  for i in {1..120}; do
    CURIO_OPERATOR_POD=$(kubectl get pods -n operators --selector="name=$POD_NAME" -o name)
    if [ -z "$CURIO_OPERATOR_POD" ]; then
      sleep 1
    else
        break
    fi
  done
}

kubectl create ns test

# kube_setup.sh
print_start_message "Running kube-setup.sh"
KUBE_SETUP_PATH=$1
CURRENT_DIR=$(pwd)
mkdir /tmp/kubesetup
cd /tmp/kubesetup || exit 1

if [ -z "$KUBE_SETUP_PATH" ]; then
  echo "Using default kube_setup from github"
  curl https://raw.githubusercontent.com/RedHatInsights/clowder/master/build/kube_setup.sh -o /tmp/kubesetup/kube_setup.sh && chmod +x /tmp/kubesetup/kube_setup.sh
  sed -i 's/^install_xjoin_operator//g' ./kube_setup.sh
  sed -i 's/^install_cyndi_operator//g' ./kube_setup.sh
  sed -i 's/^install_keda_operator//g' ./kube_setup.sh
  /tmp/kubesetup/kube_setup.sh
else
  echo "Using local kube_setup: $KUBE_SETUP_PATH"
  eval "$KUBE_SETUP_PATH"
fi
rm -r /tmp/kubesetup
cd "$CURRENT_DIR" || exit 1

kubectl set env deployment/strimzi-cluster-operator -n strimzi STRIMZI_IMAGE_PULL_SECRETS=cloudservices-pull-secret

# clowder CRDs
print_start_message "Installing Clowder CRDs"
kubectl apply -f https://github.com/RedHatInsights/clowder/releases/download/v0.21.0/clowder-manifest-v0.21.0.yaml --validate=false

# project and secrets
print_start_message "Setting up pull secrets"
dev/setup.sh --secret --project test

# operator CRDs, configmap, XJoinPipeline
print_start_message "Setting up XJoin operator"
dev/setup.sh --xjoin-operator --dev --project test

sleep 5

# bonfire environment (kafka, connect, etc.)
print_start_message "Setting up bonfire environment"
bonfire deploy-env -n test -u cloudservices -f ./dev/clowdenv.yaml

sleep 5

# inventory resources
print_start_message "Setting up host-inventory"
bonfire deploy host-inventory -n test
kubectl wait --for=condition=ContainersReady=True --selector="app=host-inventory,service=db" pods -n test

dev/forward-ports-clowder.sh test
HBI_USER=$(kubectl -n test get secret/host-inventory-db -o custom-columns=:data.username | base64 -d)
HBI_NAME=$(kubectl -n test get secret/host-inventory-db -o custom-columns=:data.name | base64 -d)
psql -U "$HBI_USER" -h inventory-db -p 5432 -d "$HBI_NAME" -c "CREATE USER insights WITH PASSWORD 'insights' SUPERUSER;"
psql -U "$HBI_USER" -h inventory-db -p 5432 -d "$HBI_NAME" -c "ALTER ROLE insights REPLICATION LOGIN;"
psql -U "$HBI_USER" -h inventory-db -p 5432 -d "$HBI_NAME" -c "CREATE PUBLICATION dbz_publication FOR TABLE hosts;"
psql -U "$HBI_USER" -h inventory-db -p 5432 -d "$HBI_NAME" -c "CREATE DATABASE test WITH TEMPLATE '$HBI_NAME';"

print_start_message "Setting up elasticsearch password"
dev/setup.sh --elasticsearch --project test