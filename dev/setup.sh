cd dev || exit 1

function usage() {
  echo -e "Usage:\n"
  echo -e "setup.sh <options>\n"

  printf "%-25s %s\n" "-p, --project (required)" "Kubernetes namespace where the resources will be created"
  printf "%-25s %s\n" "-c, --clowder" "set this when deploying into a Clowder environment (changes which ports to forward)"
  printf "%-25s %s\n" "-f, --forward-ports" "kubectl port-forward to all the services"
  printf "%-25s %s\n" "-h, --help" "show this message"

  echo -e "\nAt least one of these must be selected:\n"

  printf "%-25s %s\n" "-a, --all" "setup everything"
  printf "%-25s %s\n" "-k, --kafka" "setup Kafka, KafkaConnect"
  printf "%-25s %s\n" "-i, --inventory" "setup entire host-inventory deployment (api, mq, db, etc.)"
  printf "%-25s %s\n" "-e, --elasticsearch" "setup Elasticsearch"
  printf "%-25s %s\n" "-x, --xjoin-operator" "install the xjoin operator CRDs, xjoin configmap, and create a XJoinPipeline"
  printf "%-25s %s\n" "-s, --secret" "create a secret using the local machine's .docker/config.json"
}

function args()
{
    options=$(getopt -o p:akiehpscxf --long forward-ports --long xjoin-operator --long all --long kafka --long inventory --long elasticsearch --long project: --long secret --long clowder --long help -- "$@")
    [ $? -eq 0 ] || {
        echo "Incorrect option provided"
        exit 1
    }
    eval set -- "$options"
    while true; do
        case "$1" in

        -a)
          SETUP_KAFKA=true
          SETUP_INVENTORY=true
          SETUP_ELASTICSEARCH=true
          SETUP_PULL_SECRET=true
          ;;
        --all)
          SETUP_KAFKA=true
          SETUP_INVENTORY=true
          SETUP_ELASTICSEARCH=true
          SETUP_PULL_SECRET=true
          ;;
        -k)
          SETUP_KAFKA=true
          ;;
        --kafka)
          SETUP_KAFKA=true
          ;;
        -i)
          SETUP_INVENTORY=true
          ;;
        --inventory)
          SETUP_INVENTORY=true
          ;;
        -e)
          SETUP_ELASTICSEARCH=true
          ;;
        --elasticsearch)
          SETUP_ELASTICSEARCH=true
          ;;
        -p)
          shift;
          PROJECT_NAME=$1
          ;;
        --project)
          shift;
          PROJECT_NAME=$1
          ;;
        -s)
          SETUP_PULL_SECRET=true
          ;;
        --secret)
          SETUP_PULL_SECRET=true
          ;;
        -c)
          SETUP_CLOWDER=true
          ;;
        --clowder)
          SETUP_CLOWDER=true
          ;;
        -x)
          SETUP_XJOIN_OPERATOR=true
          ;;
        --xjoin-operator)
          SETUP_XJOIN_OPERATOR=true
          ;;
        -f)
          FORWARD_PORTS=true
          ;;
        --forward-ports)
          FORWARD_PORTS=true
          ;;
        -h)
          usage
          exit 1;
          ;;
        --help)
          usage
          exit 1;
          ;;
        --)
            shift
            break
            ;;
        esac
        shift
    done
}

args $0 "$@"

if [ -z "$PROJECT_NAME" ]; then
  echo -e "ERROR: --project must be set\n"
  usage
  exit 1
fi

if [ -z "$SETUP_ALL" ] && [ -z "$SETUP_KAFKA" ] && [ -z "$SETUP_INVENTORY" ] && [ -z "$SETUP_ELASTICSEARCH" ] && [ -z "$SETUP_XJOIN_OPERATOR" ] && [ -z "$SETUP_PULL_SECRET" ]; then
  echo -e "ERROR: must select something to setup\n"
  usage
  exit 1
fi

kubectl get ns "$PROJECT_NAME" > /dev/null
NAMESPACE_EXISTS=$?
if [ $NAMESPACE_EXISTS -eq 1 ]; then
  kubectl create ns "$PROJECT_NAME"
  sleep 2
fi

#pull secret
PULL_SECRET=xjoin-pull-secret
if [ "$SETUP_PULL_SECRET" = true ] || [ "$SETUP_ALL" = true ]; then
  echo "Setting up pull secret"
  kubectl create secret generic "$PULL_SECRET" --from-file=.dockerconfigjson="$HOME/.docker/config.json" --type=kubernetes.io/dockerconfigjson -n "$PROJECT_NAME"
  kubectl patch serviceaccount default -p "{\"imagePullSecrets\": [{\"name\": \"$PULL_SECRET\"}]}" -n "$PROJECT_NAME"
fi

#kafka
if [ "$SETUP_KAFKA" = true ] || [ "$SETUP_ALL" = true ]; then
  echo "Setting up Kafka"
  kubectl create ns kafka
  kubectl apply -f cluster-operator/ -n kafka
  kubectl apply -f cluster-operator/crd/ -n kafka

  kubectl apply -f cluster-operator/020-RoleBinding-strimzi-cluster-operator.yaml -n "$PROJECT_NAME"
  kubectl apply -f cluster-operator/032-RoleBinding-strimzi-cluster-operator-topic-operator-delegation.yaml -n "$PROJECT_NAME"
  kubectl apply -f cluster-operator/031-RoleBinding-strimzi-cluster-operator-entity-operator-delegation.yaml -n "$PROJECT_NAME"

  kubectl create -n "$PROJECT_NAME" -f kafka-cluster.yml
  echo "Waiting for kafka cluster to be ready"
  kubectl wait kafka/xjoin-kafka-cluster --for=condition=Ready --timeout=300s -n "$PROJECT_NAME"
  kubectl apply -f kafka-connect.yaml -n "$PROJECT_NAME"
  sleep 10
  kubectl patch serviceaccount xjoin-kafka-connect-strimzi-connect -p '{"imagePullSecrets": [{"name": "$PULL_SECRET"}]}' -n "$PROJECT_NAME"
  sleep 5
  kubectl get KafkaConnect
  kubectl scale --replicas=1 kafkaconnect/xjoin-kafka-connect-strimzi -n "$PROJECT_NAME"
  kubectl scale --replicas=1 deployments/xjoin-kafka-connect-strimzi-connect -n "$PROJECT_NAME"
  sleep 5
  echo "Waiting for connect to be ready"
  kubectl wait kafkaconnect/xjoin-kafka-connect-strimzi --for=condition=Ready --timeout=150s -n "$PROJECT_NAME"
  kubectl apply -f kafka-connect-topics.yaml

  sleep 10
fi

#inventory
if [ "$SETUP_INVENTORY" = true ] || [ "$SETUP_ALL" = true ]; then
  echo "Setting up host inventory"
  kubectl apply -f inventory-db.secret.yml -n "$PROJECT_NAME"
  kubectl apply -f inventory-db.yaml -n "$PROJECT_NAME"
  echo "Waiting for inventory db to be ready"
  kubectl wait deployment/inventory-db --for=condition=Available --timeout=150s -n "$PROJECT_NAME"

  kubectl apply -f inventory-migration.yml -n "$PROJECT_NAME"
  kubectl apply -f inventory-api.yml -n "$PROJECT_NAME"

  echo "Waiting for inventory-mq-pmin to be ready"
  kubectl wait jobs/inventory-migration --for=condition=Complete
  echo "Waiting for insights-inventory to be ready"
  kubectl wait deployment/insights-inventory --for=condition=Available --timeout=150s -n "$PROJECT_NAME"

  pkill -f "kubectl port-forward svc/inventory-db"
  kubectl port-forward svc/inventory-db 5432:5432 -n "$PROJECT_NAME" &
  sleep 3
  psql -U postgres -h inventory-db -p 5432 -d insights -c "ALTER ROLE postgres REPLICATION LOGIN;"
  psql -U postgres -h inventory-db -p 5432 -d insights -c "ALTER ROLE insights REPLICATION LOGIN;"
  psql -U postgres -h inventory-db -p 5432 -d insights -c "CREATE PUBLICATION dbz_publication FOR TABLE hosts;"
  psql -U postgres -h inventory-db -p 5432 -d insights -c "ALTER SYSTEM SET wal_level = logical;"
  psql -U postgres -h inventory-db -p 5432 -d insights -c "CREATE DATABASE test WITH TEMPLATE insights;"
  kubectl scale --replicas=0 deployment/inventory-db -n "$PROJECT_NAME"
  sleep 1
  kubectl scale --replicas=1 deployment/inventory-db -n "$PROJECT_NAME"
  echo "Waiting for inventory-db to be ready"
  kubectl wait deployment/inventory-db --for=condition=Available --timeout=150s -n "$PROJECT_NAME"
fi

#elasticsearch
if [ "$SETUP_ELASTICSEARCH" = true ] || [ "$SETUP_ALL" = true ]; then
  echo "Setting up Elasticsearch"
  if [ "$SETUP_CLOWDER" = true ]; then
    kubectl apply -f elasticsearch.secret.clowder.yml -n "$PROJECT_NAME"
  else
    kubectl apply -f elasticsearch.secret.yml -n "$PROJECT_NAME"
    kubectl apply -f https://download.elastic.co/downloads/eck/1.3.0/all-in-one.yaml
    sleep 10
  fi

  kubectl create -n "$PROJECT_NAME" -f elasticsearch.yml
  sleep 60
  echo "Waiting for xjoin-elasticsearch to be ready"
  kubectl wait pods/xjoin-elasticsearch-es-default-0 --for=condition=Ready --timeout=150s -n "$PROJECT_NAME"

  ES_PASSWORD=$(kubectl get secret xjoin-elasticsearch-es-elastic-user \
              -n "$PROJECT_NAME" \
              -o go-template='{{.data.elastic | base64decode}}')

  if [[ $? -eq 1 ]]; then
    echo "Unable to get ES_PASSWORD"
  fi

  pkill -f "kubectl port-forward svc/xjoin-elasticsearch-es-http"
  kubectl port-forward svc/xjoin-elasticsearch-es-http 9200:9200 -n "$PROJECT_NAME" &
  sleep 3

  curl -X POST -u "elastic:$ES_PASSWORD" -k "http://localhost:9200/_security/user/xjoin" \
       --data '{"password": "xjoin1337", "roles": ["superuser"]}' \
       -H "Content-Type: application/json"

  curl -X POST -u "elastic:$ES_PASSWORD" -k "http://localhost:9200/_security/user/test" \
       --data '{"password": "test1337", "roles": ["superuser"]}' \
       -H "Content-Type: application/json"
fi

#xjoin-operator
if [ "$SETUP_XJOIN_OPERATOR" = true ] || [ "$SETUP_ALL" = true ]; then
  cd ../
  if [ "$SETUP_CLOWDER" = true ]; then
    kubectl apply -f dev/xjoin.configmap.clowder.yaml -n "$PROJECT_NAME"
  else
    kubectl apply -f dev/xjoin.configmap.yaml -n "$PROJECT_NAME"
  fi
  make install
  kubectl apply -f config/samples/xjoin_v1alpha1_xjoinpipeline.yaml -n "$PROJECT_NAME"
fi

#forward-ports
if [ "$FORWARD_PORTS" = true ] || [ "$SETUP_ALL" = true ]; then
  if [ "$SETUP_CLOWDER" = true ]; then
    ./dev/forward-ports-clowder.sh "$PROJECT_NAME"
  else
    ./dev/forward-ports.sh "$PROJECT_NAME"
  fi
fi

echo "Done."
