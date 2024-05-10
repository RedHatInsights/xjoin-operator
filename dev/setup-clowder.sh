#!/bin/bash

set -e

INPUT=$1
HBI_AND_XJOIN="host-inventory"

if [[ "$INPUT" == "$HBI_AND_XJOIN" ]]; then
  echo "Installing host-inventory, xjoin-search and dependencies for sychronizing the host-inventory database and elasticsearch index"
elif [[ "$INPUT" ]]; then
  echo "\"$INPUT\" is not a valid argument."
  echo "Provide \"$HBI_AND_XJOIN\" as argument for \"setup-clowder.sh\" for deploying host-inventory and xjoin OR no arguments for all components."
  exit 1
else
  echo "Installing all the components specified in setup-clowder.sh"
fi

# make sure the internal network is reachable
#RESPONSE_CODE=$(curl --write-out %{http_code} --silent --output /dev/null app-interface.apps.appsrep05ue1.zqxk.p1.openshiftapps.com)
#
#if [[ "$RESPONSE_CODE" -gt 399 ]]; then
#  echo -e "Response code from app-interface: ${RESPONSE_CODE}"
#  echo -e "Unable to connect to app-interface. Are you in the VPN?"
#  exit 1
#fi


PLATFORM=`uname -a | cut -f1 -d' '`

function print_message() {
  echo -e "\n********************************************************************************"
  echo -e "* $1"
  echo -e "********************************************************************************\n"
}

function wait_for_resource_to_be_created() {
  TYPE=$1
  SELECTOR=$2
  NAMESPACE=$3

  echo -e "waiting for $TYPE to be created: $SELECTOR in namespace $NAMESPACE"
  # shellcheck disable=SC2034
  for i in {1..240}; do
    POD=$(kubectl get "$TYPE" --selector="$SELECTOR" -o name -n "$NAMESPACE")
    if [ -z "$POD" ]; then
      sleep 1
    else
        echo -e "resource created: $SELECTOR"
        break
    fi
  done
}

function wait_for_pod_to_be_running() {
  SELECTOR=$1
  NAMESPACE=$2

  if [ -z "$NAMESPACE" ]; then
    NAMESPACE="test"
  fi

  wait_for_resource_to_be_created pods "$SELECTOR" "$NAMESPACE"

  echo -e "waiting for resource to be ready: $SELECTOR in namespace $NAMESPACE"
  kubectl wait --for=condition=ContainersReady=True --selector="$SELECTOR" pods -n "$NAMESPACE" --timeout=600s
}

function wait_for_db_to_be_accessible {
    DB_HOST=$1
    DB_PORT=$2
    DB_USER=$3
    DB_NAME=$4

    echo -e "waiting for db to be accessible $DB_HOST"

    DB_IS_ACCESSIBLE=false

    # shellcheck disable=SC2034
    for i in {1..120}; do
      set +e
      if psql -U "$DB_USER" -h "$DB_HOST" -p "$DB_PORT" -d "$DB_NAME" -c "\d"; then
        set -e
  	  DB_IS_ACCESSIBLE=true
        break
      fi
      sleep 1
    done

    if [ "$DB_IS_ACCESSIBLE" = false ]; then
      echo -e "Database $DB_HOST did not become accessible"
      exit 1
    fi
    echo -e "Database $DB_HOST is ready"
}

function update_resource_spec {
    RESOURCE=$1
    JQ_STATEMENT=$2
    NAMESPACE=$3

    UPDATED=false

    # shellcheck disable=SC2034
    for i in {1..15}; do
      set +e
      if kubectl get "$RESOURCE" -o=json -n test | jq "$JQ_STATEMENT" | kubectl apply -n "$NAMESPACE" -f -; then
        set -e
        UPDATED=true
        break
      fi
      sleep 1
    done

    if [ "$UPDATED" = false ]; then
      echo -e "Unable to update resource $RESOURCE"
      exit 1
    fi
    echo -e "Updated resource $RESOURCE"
}

kubectl create ns test

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
    sed -i '' 's/^install_xjoin_operator//g' ./kube_setup.sh
    sed -i '' 's/^install_keda_operator//g' ./kube_setup.sh
  else
    sed -i 's/^install_xjoin_operator//g' ./kube_setup.sh
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
wait_for_pod_to_be_running operator-name=clowder clowder-system
sleep 5

kubectl set resources deployments --all --requests 'cpu=10m,memory=16Mi' -n test
kubectl set resources deployments --all --requests 'cpu=10m,memory=16Mi' -n cert-manager
kubectl set resources deployments --all --requests 'cpu=10m,memory=16Mi' -n clowder-system
kubectl set resources deployments --all --requests 'cpu=10m,memory=16Mi' -n cyndi-operator-system
kubectl set resources deployments --all --requests 'cpu=10m,memory=16Mi' -n default
kubectl set resources deployments --all --requests 'cpu=10m,memory=16Mi' -n elastic-system
kubectl set resources deployments --all --requests 'cpu=10m,memory=16Mi' -n kube-node-lease
kubectl set resources deployments --all --requests 'cpu=10m,memory=16Mi' -n kube-public
kubectl set resources deployments --all --requests 'cpu=10m,memory=16Mi' -n kube-system
kubectl set resources deployments --all --requests 'cpu=10m,memory=16Mi' -n strimzi

# floorist CRDs
if [[ "$INPUT" != "$HBI_AND_XJOIN" ]]; then
  kubectl apply -k https://github.com/RedHatInsights/floorist-operator/config/crd?ref=main
fi

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

# inventory resources
print_message "Setting up host-inventory"
bonfire process host-inventory -n test --no-get-dependencies --remove-dependencies all | oc apply -f - -n test

# xjoin resources
bonfire process xjoin -n test --no-get-dependencies -p xjoin-search/ES_MEMORY_REQUESTS=256Mi -p xjoin-search/ES_MEMORY_LIMITS=512Mi -p xjoin-search/ES_JAVA_OPTS="-Xms128m -Xmx128m" | oc apply -f - -n test
print_message "xjoin deployed!!!"

if [[ "$INPUT" != "$HBI_AND_XJOIN" ]]; then
  # advisor
  print_message "Setting up advisor"
  bonfire process advisor -n test --no-get-dependencies --remove-dependencies all| oc apply -f - -n test
fi

print_message "Waiting for pods to start"

# wait for the kafka pods to start
wait_for_pod_to_be_running strimzi.io/cluster=kafka,strimzi.io/kind=Kafka,strimzi.io/name=kafka-zookeeper
wait_for_pod_to_be_running strimzi.io/cluster=kafka,strimzi.io/kind=Kafka,strimzi.io/name=kafka-kafka
wait_for_pod_to_be_running strimzi.io/cluster=kafka,strimzi.io/name=kafka-entity-operator
wait_for_pod_to_be_running strimzi.io/cluster=connect

# wait for the hbi pods to start
wait_for_pod_to_be_running app=host-inventory,service=db

# wait for the xjoin pods to start
wait_for_pod_to_be_running elasticsearch.k8s.elastic.co/cluster-name=xjoin-elasticsearch
wait_for_pod_to_be_running pod=xjoin-search-api
wait_for_pod_to_be_running pod=xjoin-apicurio-service
wait_for_pod_to_be_running app=xjoin-apicurio,service=db
wait_for_pod_to_be_running app=xjoin-api-gateway

if [[ "$INPUT" != "$HBI_AND_XJOIN" ]]; then
  # wait for the advisor pods to start
  wait_for_pod_to_be_running app=advisor-backend,service=db
fi

print_message "Cleaning up extra resources"
# scale down clowder
kubectl scale --replicas=0 deployments/clowder-controller-manager -n clowder-system

# reduce memory requests
update_resource_spec kafka/kafka '.spec.entityOperator.topicOperator.resources.requests.memory = "32Mi"' test
update_resource_spec kafka/kafka '.spec.entityOperator.topicOperator.resources.limits.memory = "256Mi"' test

update_resource_spec kafka/kafka '.spec.entityOperator.userOperator.resources.requests.memory = "32Mi"' test
update_resource_spec kafka/kafka '.spec.entityOperator.userOperator.resources.limits.memory = "256Mi"' test

update_resource_spec kafka/kafka '.spec.entityOperator.tlsSidecar.resources.requests.memory = "32Mi"' test
update_resource_spec kafka/kafka '.spec.entityOperator.tlsSidecar.resources.limits.memory = "256Mi"' test

update_resource_spec kafka/kafka '.spec.zookeeper.resources.requests.memory = "128Mi"' test
update_resource_spec kafka/kafka '.spec.zookeeper.resources.limits.memory = "256Mi"' test

update_resource_spec elasticsearch/xjoin-elasticsearch '.spec.nodeSets[0].podTemplate.spec.containers[0].resources.requests.memory= "128Mi"' test
update_resource_spec elasticsearch/xjoin-elasticsearch '.spec.nodeSets[0].podTemplate.spec.containers[0].resources.limits.memory= "512Mi"' test

if [[ "$INPUT" != "$HBI_AND_XJOIN" ]]; then
  # delete extra advisor deployments, only the DB is required
  kubectl delete deployments/advisor-backend-api -n test
  kubectl delete deployments/advisor-backend-service -n test
  kubectl delete deployments/advisor-backend-tasks-service -n test
  kubectl delete deployments/host-inventory-service -n test
fi

# delete all the jobs
kubectl delete cronjobs --all -n test
kubectl delete jobs --all -n test

# setup xjoin.v2 resources, remove v1 resource
kubectl delete xjoinpipeline --all -n test
kubectl apply -f config/samples/xjoin_v1alpha1_xjoindatasource.yaml -n test
kubectl apply -f config/samples/xjoin_v1alpha1_xjoinindex.yaml -n test

# create a custom apicurio service/port to avoid conflicts
kubectl apply -f dev/apicurio.yaml -n test

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

kubectl delete pods --selector='job=host-inventory-synchronizer' -n test
kubectl delete pods --selector='job=host-inventory-org-id-populator' -n test

dev/forward-ports-clowder.sh test

END_TIME=`date +%s`
DEPLOYMENT_TIME=`expr $END_TIME - $START_TIME`
print_message "Deployment of strimzi kafka, host-inventory, and xjoin-operator took $DEPLOYMENT_TIME seconds"

print_message "Done!"
