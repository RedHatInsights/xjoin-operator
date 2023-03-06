#!/bin/bash

# make sure the internal network is reachable
RESPONSE_CODE=$(curl --write-out %{http_code} --silent --output /dev/null app-interface.apps.appsrep05ue1.zqxk.p1.openshiftapps.com)

if [[ "$RESPONSE_CODE" -gt 399 ]]; then
  echo -e "Response code from app-interface: ${RESPONSE_CODE}"
  echo -e "Unable to connect to app-interface. Are you in the VPN?"
  exit 1
fi


INCLUDE_EXTRA_STUFF=$1
PLATFORM=`uname -a | cut -f1 -d' '`

function print_message() {
  echo -e "\n********************************************************************************"
  echo -e "* $1"
  echo -e "********************************************************************************\n"
}

function wait_for_pod_to_be_running() {
  SELECTOR=$1
  echo -e "waiting for resource to be created: $SELECTOR"
  # shellcheck disable=SC2034
  for i in {1..240}; do
    POD=$(kubectl get pods --selector="$SELECTOR" -o name -n test)
    if [ -z "$POD" ]; then
      sleep 1
    else
        echo -e "resource created: $SELECTOR"
        break
    fi
  done

  echo -e "waiting for resource to be ready: $SELECTOR"
  kubectl wait --for=condition=ContainersReady=True --selector="$SELECTOR" pods -n test --timeout=600s
}

function delete_clowdapp_dependencies() {
  CLOWDAPP=$1
  echo -e "deleting dependencies of clowdapp/$CLOWDAPP"

  # shellcheck disable=SC2034
  for i in {1..60}; do
    if kubectl get clowdapp/"$CLOWDAPP" -o=json -n test | jq '.spec.dependencies = null' | kubectl apply -n test -f -; then
      break
    fi
    sleep 1
  done
}

function wait_for_db_to_be_accessible {
  DB_HOST=$1
  DB_PORT=$2
  DB_USER=$3
  DB_NAME=$4

  echo -e "waiting for db to be accessible $DB_HOST"

  # shellcheck disable=SC2034
  for i in {1..60}; do
    if psql -U "$DB_USER" -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -c "\list"; then
      break
    fi
    sleep 1
  done
}

kubectl create ns test

if [ "$INCLUDE_EXTRA_STUFF" = true ]; then
  # OLM
  print_message "Installing OLM"
  CURRENT_DIR=$(pwd)
  mkdir /tmp/olm
  cd /tmp/olm || exit 1
  curl -L https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.21.2/install.sh -o install.sh
  chmod +x install.sh
  ./install.sh v0.21.2
  rm -r /tmp/olm
  cd "$CURRENT_DIR" || exit 1
fi

# kube_setup.sh
print_message "Running kube-setup.sh"
CURRENT_DIR=$(pwd)
mkdir /tmp/kubesetup
cd /tmp/kubesetup || exit 1
START_TIME=`date +%s`

if [ -z "$KUBE_SETUP_PATH" ]; then
  echo "Using default kube_setup from github"
  curl https://raw.githubusercontent.com/RedHatInsights/clowder/master/build/kube_setup.sh -o /tmp/kubesetup/kube_setup.sh && chmod +x /tmp/kubesetup/kube_setup.sh

  if [[ $PLATFORM == "Darwin" ]]; then
    if [ "$INCLUDE_EXTRA_STUFF" = true ]; then
      sed -i '' 's/^install_xjoin_operator//g' ./kube_setup.sh
    fi
    sed -i '' 's/^install_keda_operator//g' ./kube_setup.sh
  else
    if [ "$INCLUDE_EXTRA_STUFF" = true ]; then
      sed -i 's/^install_xjoin_operator//g' ./kube_setup.sh
    fi
    sed -i 's/^install_keda_operator//g' ./kube_setup.sh
    sed -i 's/minikube kubectl --/kubectl/g' ./kube_setup.sh # this allows connecting to remote minikube instances
  fi

  /tmp/kubesetup/kube_setup.sh
fi

rm -r /tmp/kubesetup
cd "$CURRENT_DIR" || exit 1

kubectl set env deployment/strimzi-cluster-operator -n strimzi STRIMZI_IMAGE_PULL_SECRETS=cloudservices-pull-secret

# clowder CRDs
print_message "Installing Clowder CRDs"
kubectl apply -f $(curl https://api.github.com/repos/RedHatInsights/clowder/releases/latest | jq '.assets[0].browser_download_url' -r) --validate=false

# project and secrets
print_message "Setting up pull secrets"
dev/setup.sh -s -p test

# operator CRDs, configmap, XJoinPipeline
print_message "Setting up XJoin operator"
dev/setup.sh -x -d -p test
kubectl apply -f dev/xjoin-generic.configmap.yaml -n test

# bonfire environment (kafka, connect, etc.)
print_message "Setting up bonfire environment"
bonfire process-env -n test -u cloudservices --template-file ./dev/clowdenv.yaml | oc apply -f - -n test
wait_for_pod_to_be_running strimzi.io/cluster=kafka,strimzi.io/kind=Kafka,strimzi.io/name=kafka-zookeeper
wait_for_pod_to_be_running strimzi.io/cluster=kafka,strimzi.io/kind=Kafka,strimzi.io/name=kafka-kafka
wait_for_pod_to_be_running strimzi.io/cluster=connect

# inventory resources
print_message "Setting up host-inventory"
bonfire process host-inventory -n test --no-get-dependencies | oc apply -f - -n test
delete_clowdapp_dependencies host-inventory
wait_for_pod_to_be_running app=host-inventory,service=db
# xjoin resources
bonfire process xjoin -n test --no-get-dependencies | oc apply -f - -n test
wait_for_pod_to_be_running elasticsearch.k8s.elastic.co/cluster-name=xjoin-elasticsearch
wait_for_pod_to_be_running pod=xjoin-search-api

dev/forward-ports-clowder.sh test
HBI_USER=$(kubectl -n test get secret/host-inventory-db -o custom-columns=:data.username | base64 -d)
HBI_NAME=$(kubectl -n test get secret/host-inventory-db -o custom-columns=:data.name | base64 -d)
wait_for_db_to_be_accessible inventory-db 5432 "$HBI_USER" "$HBI_NAME"
psql -U "$HBI_USER" -h host-inventory-db.test.svc -p 5432 -d "$HBI_NAME" -c "CREATE USER insights WITH PASSWORD 'insights' SUPERUSER;"
psql -U "$HBI_USER" -h host-inventory-db.test.svc -p 5432 -d "$HBI_NAME" -c "ALTER ROLE insights REPLICATION LOGIN;"
psql -U "$HBI_USER" -h host-inventory-db.test.svc -p 5432 -d "$HBI_NAME" -c "CREATE PUBLICATION dbz_publication FOR TABLE hosts;"
psql -U "$HBI_USER" -h host-inventory-db.test.svc -p 5432 -d "$HBI_NAME" -c "CREATE DATABASE test WITH TEMPLATE '$HBI_NAME';"

kubectl apply -f ./dev/kafka.service.yaml -n test

# elasticsearch
print_message "Setting up elasticsearch password"
dev/setup.sh -e -p test

if [ "$INCLUDE_EXTRA_STUFF" = true ]; then
  bonfire process xjoin-api-gateway -n test | oc apply -f - -n test

  # APICurio (the ApiCurio operator has not been released in over a year, this will manually create a deployment/service)
  print_message "Installing Apicurio"
  wait_for_pod_to_be_running pod=xjoin-apicurio-service
  wait_for_pod_to_be_running app=xjoin-apicurio,service=db

  # XJoin API Gateway
  print_message "Setting up xjoin-api-gateway"
  wait_for_pod_to_be_running app=xjoin-api-gateway
fi

kubectl delete pods --selector='job=host-inventory-synchronizer' -n test
kubectl delete pods --selector='job=host-inventory-org-id-populator' -n test

dev/forward-ports-clowder.sh test

END_TIME=`date +%s`
DEPLOYMENT_TIME=`expr $END_TIME - $START_TIME`
print_message "Deployment of strimzi kafka, host-inventory, and xjoin-operator took $DEPLOYMENT_TIME seconds"

print_message "Done!"
