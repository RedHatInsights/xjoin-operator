#!/bin/bash
kubectl delete ns test
kubectl create ns test
kubectl apply -f ~/ckyrouac-secret.yml -n test
kubectl apply -f ~/ckyrouac-cloudservices-secret.yml -n test
bonfire deploy-env -n test
