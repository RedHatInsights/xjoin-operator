#!/bin/bash
ES_PASSWORD=$(kubectl get secret/xjoin-elasticsearch-es-elastic-user -o custom-columns=:data.elastic | base64 -d)
ES_INDEX=$(curl -u "elastic:$ES_PASSWORD" http://localhost:9200/_cat/indices\?format=json | jq '.[] | .index' | grep xjoinindexpipeline)
ES_INDEX=${ES_INDEX:1:-1}
export ES_INDEX