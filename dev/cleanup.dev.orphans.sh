#!/bin/bash
kubectl get xjoindatasourcepipeline -o custom-columns=name:metadata.name --no-headers | while read -r datasourcepipeline ; do
  kubectl patch xjoindatasourcepipeline "$datasourcepipeline" -p '{"metadata":{"finalizers":null}}' --type=merge
  kubectl delete xjoindatasourcepipeline "$datasourcepipeline"
done

kubectl get xjoindatasource -o custom-columns=name:metadata.name --no-headers | while read -r datasource ; do
  kubectl patch xjoindatasource "$datasource" -p '{"metadata":{"finalizers":null}}' --type=merge
  kubectl delete xjoindatasource "$datasource"
done

kubectl get xjoinindex -o custom-columns=name:metadata.name --no-headers | while read -r index ; do
  kubectl patch xjoinindex "$index" -p '{"metadata":{"finalizers":null}}' --type=merge
  kubectl delete xjoinindex "$index"
done

kubectl get xjoinindexpipeline -o custom-columns=name:metadata.name --no-headers | while read -r indexpipeline ; do
  kubectl patch xjoinindexpipeline "$indexpipeline" -p '{"metadata":{"finalizers":null}}' --type=merge
  kubectl delete xjoinindexpipeline "$indexpipeline"
done

kubectl get deployments -n test -o custom-columns=name:metadata.name --no-headers | grep xjoin-core | while read -r xjoincore ; do
  kubectl delete deployment "$xjoincore"
done

echo "Deleting connectors.."
kubectl -n test get KafkaConnector -o custom-columns=name:metadata.name | grep indexpipeline | while read -r connector ; do
    kubectl delete KafkaConnector "$connector" -n test
done
kubectl -n test get KafkaConnector -o custom-columns=name:metadata.name | grep datasourcepipeline | while read -r connector ; do
    kubectl delete KafkaConnector "$connector" -n test
done

echo "Deleting topics.."
kubectl -n test get KafkaTopic -o custom-columns=name:metadata.name | grep indexpipeline | while read -r topic ; do
    kubectl delete KafkaTopic "$topic" -n test
done
kubectl -n test get KafkaTopic -o custom-columns=name:metadata.name | grep datasourcepipeline | while read -r topic ; do
    kubectl delete KafkaTopic "$topic" -n test
done

echo "Deleting avro subjects.."
curl localhost:8081/subjects | jq -c '.[]' | while read i; do
    i="${i:1}"
    i="${i::-1}"
    echo "$i"
    curl -X DELETE localhost:8081/subjects/$i
done

# do this twice in case a referenced schema is deleted too early
curl localhost:8081/subjects | jq -c '.[]' | while read i; do
    i="${i:1}"
    i="${i::-1}"
    echo "$i"
    curl -X DELETE localhost:8081/subjects/$i
done

HBI_USER=$(kubectl get secret/host-inventory-db -o custom-columns=:data.username | base64 -d)
HBI_NAME=$(kubectl get secret/host-inventory-db -o custom-columns=:data.name | base64 -d)
psql -U "$HBI_USER" -h inventory-db -p 5432 -d "$HBI_NAME" -t -c "SELECT slot_name from pg_catalog.pg_replication_slots" | while read -r slot ; do
  psql -U "$HBI_USER" -h inventory-db -p 5432 -d "$HBI_NAME" -c "SELECT pg_drop_replication_slot('$slot');"
done