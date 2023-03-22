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

kubectl get xjoinindexvalidator -o custom-columns=name:metadata.name --no-headers | while read -r indexvalidator ; do
  kubectl patch xjoinindexvalidator "$indexvalidator" -p '{"metadata":{"finalizers":null}}' --type=merge
  kubectl delete xjoinindexvalidator "$indexvalidator"
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
APICURIO_HOSTNAME=apicurio.test.svc
APICURIO_PORT=10001
artifacts=$(curl "http://$APICURIO_HOSTNAME:$APICURIO_PORT/apis/registry/v2/search/artifacts?limit=100" | jq '.artifacts|map(.id)|@sh')
artifacts=($artifacts)
total=${#artifacts[@]}
for i in "${!artifacts[@]}"; do
    if [ "$total" -eq 1 ]; then
      artifact=${artifacts[$i]}
      artifact="${artifact:2}"
      artifact="${artifact::-2}"
    elif [ "$i" -eq 0 ]; then
      artifact=${artifacts[$i]}
      artifact="${artifact:2}"
      artifact="${artifact::-1}"
    elif [ "$i" -eq $(("$total-1")) ]; then
      artifact=${artifacts[$i]}
      artifact="${artifact:1}"
      artifact="${artifact::-2}"
    else
      artifact=${artifacts[$i]}
      artifact="${artifact:1}"
      artifact="${artifact::-1}"
    fi
    echo "$artifact"
    curl -X DELETE "http://$APICURIO_HOSTNAME:$APICURIO_PORT/apis/registry/v1/artifacts/$artifact"
done

echo "Deleting replication slots"
HBI_USER=$(kubectl get secret/host-inventory-db -o custom-columns=:data.username | base64 -d)
HBI_NAME=$(kubectl get secret/host-inventory-db -o custom-columns=:data.name | base64 -d)
HBI_HOSTNAME=host-inventory-db.test.svc
psql -U "$HBI_USER" -h "$HBI_HOSTNAME" -p 5432 -d "$HBI_NAME" -t -c "SELECT slot_name from pg_catalog.pg_replication_slots" | while read -r slot ; do
  psql -U "$HBI_USER" -h "$HBI_HOSTNAME" -p 5432 -d "$HBI_NAME" -c "SELECT pg_drop_replication_slot('$slot');"
done

echo "Deleting ES indexes"
ES_PASSWORD=$(kubectl get secret/xjoin-elasticsearch-es-elastic-user -o custom-columns=:data.elastic | base64 -d)
ES_HOSTNAME=xjoin-elasticsearch-es-default.test.svc
curl -u "elastic:$ES_PASSWORD" "http://$ES_HOSTNAME:9200/_cat/indices?format=json" | jq '.[] | .index' | grep xjoinindexpipeline | while read -r index ; do
  index="${index:1}"
  index="${index::-1}"
  curl -u "elastic:$ES_PASSWORD" -X DELETE "http://$ES_HOSTNAME:9200/$index"
done

echo "Deleting subgraph pods"
kubectl delete deployments --selector='xjoin.index=xjoinindexpipeline-hosts'
kubectl delete deployments --selector='xjoin.index=xjoinindexpipeline-cats'
kubectl delete deployments --selector='xjoin.index=xjoinindexpipeline-cats'
kubectl delete deployments --selector='xjoin.index=xjoinindexpipeline-hosts-hbi-tags'

kubectl delete services --selector='xjoin.index=xjoinindexpipeline-hosts'
kubectl delete services --selector='xjoin.index=xjoinindexpipeline-cats'
kubectl delete services --selector='xjoin.index=xjoinindexpipeline-cats'
kubectl delete services --selector='xjoin.index=xjoinindexpipeline-hosts-hbi-tags'

kubectl delete pods --selector='xjoin.component.name=XJoinIndexValidator'
