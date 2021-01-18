#!/bin/bash
oc get projects -o custom-columns=name:metadata.name | grep xjointest | while read line ; do
    echo $line
    oc project $line && kubectl get XJoinPipeline test-pipeline-01 -o=json | jq '.metadata.finalizers = null' | kubectl apply -f -
done
