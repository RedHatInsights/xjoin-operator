#!/bin/bash
echo "Deleting topics.."
kubectl get KafkaTopic -o custom-columns=name:metadata.name | grep xjointest | while read -r topic ; do
    kubectl delete KafkaTopic "$topic"
done

echo "Deleting connectors.."
kubectl get KafkaConnector -o custom-columns=name:metadata.name | grep xjointest | while read -r connector ; do
    kubectl delete KafkaConnector "$connector"
done

echo "Deleting namespaces.."
kubectl get namespaces -o custom-columns=name:metadata.name | grep xjointest | while read -r namespace ; do
    echo "$namespace"
    kubectl get XJoinPipeline test-pipeline-01 -o=json -n "$namespace" | jq '.metadata.finalizers = null' | kubectl apply -f -
    
    kubectl delete namespace "$namespace"
done

kubectl get namespaces -o custom-columns=name:metadata.name | grep test | while read -r project ; do
    echo "$project"
    kubectl get CyndiPipeline integration-test-pipeline -o=json -n "$project" | jq '.metadata.finalizers = null' | kubectl apply -f -

    kubectl delete KafkaTopic --all
    sleep 1
    kubectl delete KafkaConnector --all
    sleep 1

    kubectl delete project "$project"
done

echo "Deleting indices..."
curl -u "xjoin:xjoin1337" "http://localhost:9200/_cat/indices/xjointest*?h=index" | while read -r index ; do
  curl -u "xjoin:xjoin1337" -X DELETE "http://localhost:9200/$index"
done