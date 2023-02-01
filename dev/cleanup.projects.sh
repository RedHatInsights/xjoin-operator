#!/bin/bash
echo "Deleting connectors.."
kubectl -n test get KafkaConnector -o custom-columns=name:metadata.name | grep xjointest | while read -r connector ; do
    kubectl delete KafkaConnector "$connector" -n test
done

echo "Deleting topics.."
kubectl -n test get KafkaTopic -o custom-columns=name:metadata.name | grep xjointest | while read -r topic ; do
    kubectl delete KafkaTopic "$topic" -n test
done

echo "Deleting namespaces.."
kubectl get namespaces -o custom-columns=name:metadata.name | grep xjointest | while read -r namespace ; do
    echo "$namespace"
    kubectl patch XJoinPipeline test-pipeline-01 -n "$namespace" -p '{"metadata":{"finalizers":[]}}' --type=merge
    kubectl delete namespace "$namespace"
done

#kubectl get namespaces -o custom-columns=name:metadata.name | grep test | while read -r project ; do
#    echo "$project"
#
#    kubectl delete KafkaConnector --all -n "$project"
#    sleep 1
#    kubectl delete KafkaTopic --all -n "$project"
#    sleep 1
#done
#
#echo "Deleting indices..."
#curl -u "xjoin:xjoin1337" "http://localhost:9200/_cat/indices/xjointest*?h=index" | while read -r index ; do
#  curl -u "xjoin:xjoin1337" -X DELETE "http://localhost:9200/$index"
#done
#
#echo "Deleting topics.."
#kubectl -n test get KafkaTopic -o custom-columns=name:metadata.name | grep xjointest | while read -r topic ; do
#    kubectl delete KafkaTopic "$topic" -n test
#done
