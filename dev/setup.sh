if [[ -z "$1" ]]; then
    echo "pull secret name must be set"
    exit 1
fi

if [[ -z "$2" ]]; then
    echo "secret file location must be set"
    exit 1
fi

PROJECT_NAME=xjoin-operator-project
PULL_SECRET=$1
SECRET_FILE_LOC=$2

oc whoami || exit 1

oc create ns $PROJECT_NAME
oc apply -f "$SECRET_FILE_LOC" -n $PROJECT_NAME
oc get secret "$PULL_SECRET" -n $PROJECT_NAME || exit 1

oc secrets link -n $PROJECT_NAME default "$PULL_SECRET" --for=pull

oc create ns kafka
oc apply -f cluster-operator/ -n kafka

oc apply -f 020-RoleBinding-strimzi-cluster-operator.yaml -n $PROJECT_NAME
oc apply -f 032-RoleBinding-strimzi-cluster-operator-topic-operator-delegation.yaml -n $PROJECT_NAME
oc apply -f 031-RoleBinding-strimzi-cluster-operator-entity-operator-delegation.yaml -n $PROJECT_NAME

sleep 10

oc project $PROJECT_NAME

oc create -n $PROJECT_NAME -f kafka-cluster.yml
oc wait kafka/xjoin-kafka-cluster --for=condition=Ready --timeout=300s -n $PROJECT_NAME

oc apply -f inventory-db.secret.yml -n $PROJECT_NAME
oc apply -f inventory-db.yaml -n $PROJECT_NAME
oc wait deployment/inventory-db --for=condition=Available --timeout=300s -n $PROJECT_NAME

oc apply -f kafka-connect.yaml -n $PROJECT_NAME

sleep 1

oc secrets link xjoin-kafka-connect-strimzi-connect $PULL_SECRET --for=pull -n $PROJECT_NAME

oc scale --replicas=1 kafkaconnect xjoin-kafka-connect-strimzi
oc wait kafkaconnect/xjoin-kafka-connect-strimzi --for=condition=Ready --timeout=300s -n $PROJECT_NAME

oc apply -f https://download.elastic.co/downloads/eck/1.3.0/all-in-one.yaml
sleep 10

oc create -n $PROJECT_NAME -f elasticsearch.yml
sleep 60
oc wait pods/xjoin-elasticsearch-es-default-0 --for=condition=Ready --timeout=300s -n $PROJECT_NAME

ES_PASSWORD=$(oc get secret xjoin-elasticsearch-es-elastic-user \
            -n xjoin-operator-project \
            -o go-template='{{.data.elastic | base64decode}}')
curl -X POST -u "elastic:$ES_PASSWORD" -k "https://elasticsearch:9200/_security/user/xjoin" \
     --data '{"password": "xjoin1337", "roles": ["superuser"]}' \
     -H "Content-Type: application/json"

if [[ $? -eq 1 ]]; then
  echo "Unable to get ES_PASSWORD"
fi
