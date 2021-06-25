#!/bin/bash
oc project xjoin-operator-project

echo "Deleting topics.."
oc get KafkaTopic -o custom-columns=name:metadata.name | grep xjointest | while read topic ; do
    oc delete KafkaTopic $topic
done

echo "Deleting connectors.."
oc get KafkaConnector -o custom-columns=name:metadata.name | grep xjointest | while read connector ; do
    oc delete KafkaConnector $connector
done

echo "Deleting projects.."
oc get projects -o custom-columns=name:metadata.name | grep xjointest | while read project ; do
    echo $project
    oc project $project && kubectl get XJoinPipeline test-pipeline-01 -o=json | jq '.metadata.finalizers = null' | kubectl apply -f -
    
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

echo "Deleting indices..."
curl -u "xjoin:xjoin1337" "http://elasticsearch:9200/_cat/indices/xjointest*?h=index" | while read index ; do
  curl -u "xjoin:xjoin1337" -X DELETE "http://elasticsearch:9200/$index"
done