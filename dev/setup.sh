cd dev

PROJECT_NAME=xjoin-operator-project
PULL_SECRET=xjoin-pull-secret

kubectl create ns $PROJECT_NAME

sleep 2

kubectl create secret generic "$PULL_SECRET" --from-file=.dockercfg="$HOME/.docker/config.json" --type=kuberenetes.io/dockercfgjson -n $PROJECT_NAME
kubectl patch serviceaccount default -p "{\"imagePullSecrets\": [{\"name\": \"$PULL_SECRET\"}]}" -n $PROJECT_NAME

#kafka
kubectl create ns kafka
kubectl apply -f cluster-operator/ -n kafka
kubectl apply -f cluster-operator/crd/ -n kafka

kubectl apply -f cluster-operator/020-RoleBinding-strimzi-cluster-operator.yaml -n $PROJECT_NAME
kubectl apply -f cluster-operator/032-RoleBinding-strimzi-cluster-operator-topic-operator-delegation.yaml -n $PROJECT_NAME
kubectl apply -f cluster-operator/031-RoleBinding-strimzi-cluster-operator-entity-operator-delegation.yaml -n $PROJECT_NAME

kubectl create -n $PROJECT_NAME -f kafka-cluster.yml
echo "Waiting for kafka cluster to be ready"
kubectl wait kafka/xjoin-kafka-cluster --for=condition=Ready --timeout=300s -n $PROJECT_NAME
kubectl apply -f kafka-connect.yaml -n $PROJECT_NAME
sleep 10
kubectl patch serviceaccount xjoin-kafka-connect-strimzi-connect -p '{"imagePullSecrets": [{"name": "$PULL_SECRET"}]}' -n $PROJECT_NAME
sleep 5
kubectl get KafkaConnect
kubectl scale --replicas=1 kafkaconnect/xjoin-kafka-connect-strimzi -n $PROJECT_NAME
kubectl scale --replicas=1 deployments/xjoin-kafka-connect-strimzi-connect -n $PROJECT_NAME
sleep 5
echo "Waiting for connect to be ready"
kubectl wait kafkaconnect/xjoin-kafka-connect-strimzi --for=condition=Ready --timeout=150s -n $PROJECT_NAME
kubectl apply -f kafka-connect-topics.yaml

sleep 10

#inventory
kubectl apply -f inventory-db.secret.yml -n $PROJECT_NAME
kubectl apply -f inventory-db.yaml -n $PROJECT_NAME
echo "Waiting for inventory db to be ready"
kubectl wait deployment/inventory-db --for=condition=Available --timeout=150s -n $PROJECT_NAME

kubectl apply -f inventory-migration.yml -n $PROJECT_NAME
kubectl apply -f inventory-api.yml -n $PROJECT_NAME

echo "Waiting for inventory-mq-pmin to be ready"
kubectl wait jobs/inventory-migration --for=condition=Complete
echo "Waiting for insights-inventory to be ready"
kubectl wait deployment/insights-inventory --for=condition=Available --timeout=150s -n $PROJECT_NAME

pkill -f "kubectl port-forward svc/inventory-db"
kubectl port-forward svc/inventory-db 5432:5432 -n $PROJECT_NAME &
sleep 3
psql -U postgres -h inventory-db -p 5432 -d insights -c "ALTER ROLE postgres REPLICATION LOGIN;"
psql -U postgres -h inventory-db -p 5432 -d insights -c "ALTER ROLE insights REPLICATION LOGIN;"
psql -U postgres -h inventory-db -p 5432 -d insights -c "CREATE PUBLICATION dbz_publication FOR TABLE hosts;"
psql -U postgres -h inventory-db -p 5432 -d insights -c "ALTER SYSTEM SET wal_level = logical;"
psql -U postgres -h inventory-db -p 5432 -d insights -c "CREATE DATABASE test WITH TEMPLATE insights;"
kubectl scale --replicas=0 deployment/inventory-db -n $PROJECT_NAME
sleep 1
kubectl scale --replicas=1 deployment/inventory-db -n $PROJECT_NAME
echo "Waiting for inventory-db to be ready"
kubectl wait deployment/inventory-db --for=condition=Available --timeout=150s -n $PROJECT_NAME

#elasticsearch
kubectl apply -f elasticsearch.secret.yml -n $PROJECT_NAME
kubectl apply -f https://download.elastic.co/downloads/eck/1.3.0/all-in-one.yaml
sleep 10

kubectl create -n $PROJECT_NAME -f elasticsearch.yml
sleep 60
echo "Waiting for xjoin-elasticsearch to be ready"
kubectl wait pods/xjoin-elasticsearch-es-default-0 --for=condition=Ready --timeout=150s -n $PROJECT_NAME

ES_PASSWORD=$(kubectl get secret xjoin-elasticsearch-es-elastic-user \
            -n $PROJECT_NAME \
            -o go-template='{{.data.elastic | base64decode}}')

if [[ $? -eq 1 ]]; then
  echo "Unable to get ES_PASSWORD"
fi

pkill -f "kubectl port-forward svc/xjoin-elasticsearch-es-http"
kubectl port-forward svc/xjoin-elasticsearch-es-http 9200:9200 -n $PROJECT_NAME &
sleep 3

curl -X POST -u "elastic:$ES_PASSWORD" -k "http://localhost:9200/_security/user/xjoin" \
     --data '{"password": "xjoin1337", "roles": ["superuser"]}' \
     -H "Content-Type: application/json"

curl -X POST -u "elastic:$ES_PASSWORD" -k "http://localhost:9200/_security/user/test" \
     --data '{"password": "test1337", "roles": ["superuser"]}' \
     -H "Content-Type: application/json"

./forward-ports.sh

cd ../
kubectl apply -f dev/xjoin.configmap.yaml -n $PROJECT_NAME
make install
kubectl apply -f config/samples/xjoin_v1alpha1_xjoinpipeline.yaml -n $PROJECT_NAME

echo "Done."
