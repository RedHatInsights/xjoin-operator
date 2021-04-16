if [[ -z "$1" ]]; then
    echo "pull secret name must be set"
    exit 1
fi

if [[ -z "$2" ]]; then
    echo "secret file location must be set"
    exit 1
fi

cd dev

PROJECT_NAME=xjoin-operator-project
PULL_SECRET=$1
SECRET_FILE_LOC=$2

oc whoami || exit 1

oc create ns $PROJECT_NAME
oc apply -f "$SECRET_FILE_LOC" -n $PROJECT_NAME
oc get secret "$PULL_SECRET" -n $PROJECT_NAME || exit 1

oc secrets link -n $PROJECT_NAME default "$PULL_SECRET" --for=pull

#kafka
oc create ns kafka
oc apply -f cluster-operator/ -n kafka
oc apply -f cluster-operator/crd/ -n kafka

oc apply -f cluster-operator/020-RoleBinding-strimzi-cluster-operator.yaml -n $PROJECT_NAME
oc apply -f cluster-operator/032-RoleBinding-strimzi-cluster-operator-topic-operator-delegation.yaml -n $PROJECT_NAME
oc apply -f cluster-operator/031-RoleBinding-strimzi-cluster-operator-entity-operator-delegation.yaml -n $PROJECT_NAME

oc create -n $PROJECT_NAME -f kafka-cluster.yml
echo "Waiting for kafka cluster to be ready"
oc wait kafka/xjoin-kafka-cluster --for=condition=Ready --timeout=150s -n $PROJECT_NAME
oc apply -f kafka-connect.yaml -n $PROJECT_NAME
sleep 10
oc secrets link xjoin-kafka-connect-strimzi-connect $PULL_SECRET --for=pull -n $PROJECT_NAME
sleep 5
oc get KafkaConnect
oc scale --replicas=1 kafkaconnect/xjoin-kafka-connect-strimzi
oc scale --replicas=1 deployments/xjoin-kafka-connect-strimzi-connect
sleep 5
echo "Waiting for connect to be ready"
oc wait kafkaconnect/xjoin-kafka-connect-strimzi --for=condition=Ready --timeout=150s -n $PROJECT_NAME
oc apply -f kafka-connect-topics.yaml

sleep 10

oc project $PROJECT_NAME

#inventory
oc apply -f inventory-db.secret.yml -n $PROJECT_NAME
oc apply -f inventory-db.yaml -n $PROJECT_NAME
echo "Waiting for inventory db to be ready"
oc wait deployment/inventory-db --for=condition=Available --timeout=150s -n $PROJECT_NAME

oc apply -f inventory-mq.yml -n $PROJECT_NAME
oc apply -f inventory-api.yml -n $PROJECT_NAME

echo "Waiting for inventory-mq-pmin to be ready"
oc wait dc/inventory-mq-pmin --for=condition=Available --timeout=150s -n $PROJECT_NAME
echo "Waiting for insights-inventory to be ready"
oc wait deployment/insights-inventory --for=condition=Available --timeout=150s -n $PROJECT_NAME

pkill -f "oc port-forward svc/inventory-db"
oc port-forward svc/inventory-db 5432:5432 -n xjoin-operator-project &
sleep 3
psql -U postgres -h inventory-db -p 5432 -d insights -c "ALTER ROLE insights REPLICATION LOGIN;"
psql -U postgres -h inventory-db -p 5432 -d insights -c "CREATE PUBLICATION dbz_publication FOR TABLE hosts;"
psql -U postgres -h inventory-db -p 5432 -d insights -c "ALTER SYSTEM SET wal_level = logical;"
psql -U postgres -h inventory-db -p 5432 -d insights -c "CREATE DATABASE test WITH TEMPLATE insights;"
oc scale --replicas=0 deployment/inventory-db
sleep 1
oc scale --replicas=1 deployment/inventory-db
echo "Waiting for inventory-db to be ready"
oc wait deployment/inventory-db --for=condition=Available --timeout=150s -n $PROJECT_NAME

#elasticsearch
oc apply -f elasticsearch.secret.yml -n $PROJECT_NAME
oc apply -f https://download.elastic.co/downloads/eck/1.3.0/all-in-one.yaml
sleep 10

oc create -n $PROJECT_NAME -f elasticsearch.yml
sleep 60
echo "Waiting for xjoin-elasticsearch to be ready"
oc wait pods/xjoin-elasticsearch-es-default-0 --for=condition=Ready --timeout=150s -n $PROJECT_NAME

ES_PASSWORD=$(oc get secret xjoin-elasticsearch-es-elastic-user \
            -n xjoin-operator-project \
            -o go-template='{{.data.elastic | base64decode}}')

if [[ $? -eq 1 ]]; then
  echo "Unable to get ES_PASSWORD"
fi

pkill -f "oc port-forward svc/xjoin-elasticsearch-es-http"
oc port-forward svc/xjoin-elasticsearch-es-http 9200:9200 -n xjoin-operator-project &
sleep 3

curl -X POST -u "elastic:$ES_PASSWORD" -k "http://elasticsearch:9200/_security/user/xjoin" \
     --data '{"password": "xjoin1337", "roles": ["superuser"]}' \
     -H "Content-Type: application/json"

curl -X POST -u "elastic:$ES_PASSWORD" -k "http://elasticsearch:9200/_security/user/test" \
     --data '{"password": "test1337", "roles": ["superuser"]}' \
     -H "Content-Type: application/json"

./forward-ports.sh

cd ../
oc apply -f dev/xjoin.configmap.yaml
make install
oc apply -f config/samples/xjoin_v1alpha1_xjoinpipeline.yaml

echo "Done."
