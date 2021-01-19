#!/bin/bash
oc get projects -o custom-columns=name:metadata.name | grep xjointest | while read project ; do
    echo $project
    oc project $project && kubectl get XJoinPipeline test-pipeline-01 -o=json | jq '.metadata.finalizers = null' | kubectl apply -f -
    oc delete project $project
done

oc get projects -o custom-columns=name:metadata.name | grep test | while read project ; do
    echo $project
    oc project $project && kubectl get CyndiPipeline integration-test-pipeline -o=json | jq '.metadata.finalizers = null' | kubectl apply -f -
    oc delete project $project
done
