#!/bin/bash

./dev/cleanup.projects.sh

kubectl delete namespace cert-manager &
kubectl delete namespace cyndi-operator &
kubectl delete namespace strimzi &
kubectl delete namespace test &
kubectl delete namespace elastic-system &
kubectl delete namespace clowder-system &

wait
pkill -f "kubectl port-forward"
echo "Done"
