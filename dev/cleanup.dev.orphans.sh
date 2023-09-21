#!/bin/bash

PROJECT_NAME=$1

if [ -z "$PROJECT_NAME" ]; then
  PROJECT_NAME="test"
fi

echo "Using namespace $PROJECT_NAME"

kubectl get xjoindatasourcepipeline -o custom-columns=name:metadata.name --no-headers -n "$PROJECT_NAME" | while read -r datasourcepipeline ; do
  kubectl patch xjoindatasourcepipeline "$datasourcepipeline" -p '{"metadata":{"finalizers":null}}' --type=merge -n "$PROJECT_NAME"
  kubectl delete xjoindatasourcepipeline "$datasourcepipeline" -n "$PROJECT_NAME"
done

kubectl get xjoindatasource -o custom-columns=name:metadata.name --no-headers -n "$PROJECT_NAME" | while read -r datasource ; do
  kubectl patch xjoindatasource "$datasource" -p '{"metadata":{"finalizers":null}}' --type=merge -n "$PROJECT_NAME"
  kubectl delete xjoindatasource "$datasource" -n "$PROJECT_NAME"
done

kubectl get xjoinindex -o custom-columns=name:metadata.name --no-headers -n "$PROJECT_NAME" | while read -r index ; do
  kubectl patch xjoinindex "$index" -p '{"metadata":{"finalizers":null}}' --type=merge -n "$PROJECT_NAME"
  kubectl delete xjoinindex "$index" -n "$PROJECT_NAME"
done

kubectl get xjoinindexpipeline -o custom-columns=name:metadata.name --no-headers -n "$PROJECT_NAME" | while read -r indexpipeline ; do
  kubectl patch xjoinindexpipeline "$indexpipeline" -p '{"metadata":{"finalizers":null}}' --type=merge -n "$PROJECT_NAME"
  kubectl delete xjoinindexpipeline "$indexpipeline" -n "$PROJECT_NAME"
done

kubectl get xjoinindexvalidator -o custom-columns=name:metadata.name --no-headers -n "$PROJECT_NAME" | while read -r indexvalidator ; do
  kubectl patch xjoinindexvalidator "$indexvalidator" -p '{"metadata":{"finalizers":null}}' --type=merge -n "$PROJECT_NAME"
  kubectl delete xjoinindexvalidator "$indexvalidator" -n "$PROJECT_NAME"
done

kubectl get deployments -o custom-columns=name:metadata.name --no-headers -n "$PROJECT_NAME" | grep xjoin-core | while read -r xjoincore ; do
  kubectl delete deployment "$xjoincore" -n "$PROJECT_NAME"
done

echo "Deleting connectors.."
kubectl get KafkaConnector -o custom-columns=name:metadata.name -n "$PROJECT_NAME" | grep indexpipeline | while read -r connector ; do
    kubectl delete KafkaConnector "$connector" -n "$PROJECT_NAME"
done
kubectl get KafkaConnector -o custom-columns=name:metadata.name -n "$PROJECT_NAME" | grep datasourcepipeline | while read -r connector ; do
    kubectl delete KafkaConnector "$connector" -n "$PROJECT_NAME"
done

echo "Deleting topics.."
kubectl get KafkaTopic -o custom-columns=name:metadata.name -n "$PROJECT_NAME" | grep indexpipeline | while read -r topic ; do
    kubectl delete KafkaTopic "$topic" -n "$PROJECT_NAME"
done
kubectl get KafkaTopic -o custom-columns=name:metadata.name -n "$PROJECT_NAME" | grep datasourcepipeline | while read -r topic ; do
    kubectl delete KafkaTopic "$topic" -n "$PROJECT_NAME"
done

echo "Deleting avro subjects.."
APICURIO_HOSTNAME="xjoin-apicurio-service.$PROJECT_NAME.svc"
APICURIO_PORT=10000
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
HBI_USER=$(kubectl get secret/host-inventory-db -o custom-columns=:data.username -n "$PROJECT_NAME" | base64 -d)
HBI_NAME=$(kubectl get secret/host-inventory-db -o custom-columns=:data.name -n "$PROJECT_NAME" | base64 -d)
HBI_HOSTNAME="host-inventory-db.$PROJECT_NAME.svc"
psql -U "$HBI_USER" -h "$HBI_HOSTNAME" -p 5432 -d "$HBI_NAME" -t -c "SELECT slot_name from pg_catalog.pg_replication_slots" | while read -r slot ; do
  psql -U "$HBI_USER" -h "$HBI_HOSTNAME" -p 5432 -d "$HBI_NAME" -c "SELECT pg_drop_replication_slot('$slot');"
done

echo "Deleting ES indexes"
ES_PASSWORD=$(kubectl get secret/xjoin-elasticsearch-es-elastic-user -o custom-columns=:data.elastic -n "$PROJECT_NAME"| base64 -d)
ES_HOSTNAME="xjoin-elasticsearch-es-default.$PROJECT_NAME.svc"
curl -u "elastic:$ES_PASSWORD" "http://$ES_HOSTNAME:9200/_cat/indices?format=json" | jq '.[] | .index' | grep -e xjoinindexpipeline -e test.hosts -e xjointest -e prefix | while read -r index ; do
  index="${index:1}"
  index="${index::-1}"
  curl -u "elastic:$ES_PASSWORD" -X DELETE "http://$ES_HOSTNAME:9200/$index"
done

echo "Deleting ES pipelines"
ES_PASSWORD=$(kubectl get secret/xjoin-elasticsearch-es-elastic-user -o custom-columns=:data.elastic -n "$PROJECT_NAME"| base64 -d)
ES_HOSTNAME="xjoin-elasticsearch-es-default.$PROJECT_NAME.svc"
curl -u "elastic:$ES_PASSWORD" "http://$ES_HOSTNAME:9200/_ingest/pipeline?format=json" | jq 'keys[]' | grep -e xjoinindexpipeline -e prefix -e test.hosts -e xjointest | while read -r index ; do
  index="${index:1}"
  index="${index::-1}"
  curl -u "elastic:$ES_PASSWORD" -X DELETE "http://$ES_HOSTNAME:9200/_ingest/pipeline/$index"
done

echo "Deleting subgraph pods"
kubectl delete deployments --selector='xjoin.index=xjoinindexpipeline-hosts' -n "$PROJECT_NAME"
kubectl delete deployments --selector='xjoin.index=xjoinindexpipeline-cats' -n "$PROJECT_NAME"
kubectl delete deployments --selector='xjoin.index=xjoinindexpipeline-cats' -n "$PROJECT_NAME"
kubectl delete deployments --selector='xjoin.index=xjoinindexpipeline-hosts-hbi-tags' -n "$PROJECT_NAME"

kubectl delete deployments --selector='xjoin.component.name=XJoinAPISubgraph' -n "$PROJECT_NAME"

kubectl delete services --selector='xjoin.index=xjoinindexpipeline-hosts' -n "$PROJECT_NAME"
kubectl delete services --selector='xjoin.index=xjoinindexpipeline-cats' -n "$PROJECT_NAME"
kubectl delete services --selector='xjoin.index=xjoinindexpipeline-cats' -n "$PROJECT_NAME"
kubectl delete services --selector='xjoin.index=xjoinindexpipeline-hosts-hbi-tags' -n "$PROJECT_NAME"

kubectl delete pods --selector='xjoin.component.name=XJoinIndexValidator' -n "$PROJECT_NAME"