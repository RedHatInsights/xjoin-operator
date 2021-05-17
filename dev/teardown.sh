#!/bin/bash

./dev/cleanup.projects.sh

kubectl delete namespace xjoin-operator-project &
kubectl delete namespace kafka &
kubectl delete namespace elastic-system &

wait
pkill -f "oc port-forward"
echo "Done"
