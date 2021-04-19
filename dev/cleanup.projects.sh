#!/bin/bash
oc project default

oc get projects -o custom-columns=name:metadata.name | grep xjointest | while read project ; do
    echo $project
    oc project $project && kubectl get XJoinPipeline test-pipeline-01 -o=json | jq '.metadata.finalizers = null' | kubectl apply -f -
    
    oc delete KafkaTopic --all
    sleep 1
    oc delete KafkaConnector --all
    sleep 1

    oc delete project $project
done

oc get projects -o custom-columns=name:metadata.name | grep test | while read project ; do
    echo $project
    oc project $project && kubectl get CyndiPipeline integration-test-pipeline -o=json | jq '.metadata.finalizers = null' | kubectl apply -f -

    oc delete KafkaTopic --all
    sleep 1
    oc delete KafkaConnector --all
    sleep 1

    oc delete project $project
done

oc project xjoin-operator-project

#oc get KafkaTopic -o custom-columns=name:metadata.name | grep test | while read topic ; do
#    oc delete KafkaTopic $topic
#done
