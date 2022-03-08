#!/bin/bash

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

kubectl delete elasticsearch --all
kubectl wait --for=delete --selector="elasticsearch.k8s.elastic.co/statefulset-name=xjoin-elasticsearch-es-default" pods -n test --timeout=600s

kubectl apply -f dev/elasticsearch.yml
wait_for_pod_to_be_running "elasticsearch.k8s.elastic.co/statefulset-name=xjoin-elasticsearch-es-default"
./dev/setup.sh -e -p test
./dev/forward-ports-clowder.sh