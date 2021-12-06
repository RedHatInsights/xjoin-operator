cd dev || exit 1

function usage() {
  echo -e "Usage:\n"
  echo -e "setup.sh <options>\n"

  printf "%-25s %s\n" "-p, --project (required)" "Kubernetes namespace where the resources will be created"
  printf "%-25s %s\n" "-d, --dev" "Run make install to install the operator CRDs. Use this when running the operator locally for development."
  printf "%-25s %s\n" "-f, --forward-ports" "kubectl port-forward to all the services"
  printf "%-25s %s\n" "-h, --help" "show this message"

  echo -e "\nAt least one of these must be selected:\n"

  printf "%-25s %s\n" "-a, --all" "setup everything"
  printf "%-25s %s\n" "-e, --elasticsearch" "setup Elasticsearch"
  printf "%-25s %s\n" "-x, --xjoin-operator" "install the xjoin operator CRDs, xjoin configmap, and create a XJoinPipeline"
  printf "%-25s %s\n" "-s, --secret" "create a secret using the local machine's .docker/config.json"
}

function args()
{
    options=$(getopt -o p:adehpsxfo --long olm --long dev --long forward-ports --long xjoin-operator --long all --long elasticsearch --long project: --long secret --long help -- "$@")
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
        -d)
          SETUP_DEV=true
          ;;
        --dev)
          SETUP_DEV=true
          ;;
        -o)
          SETUP_OLM=true
          ;;
        --olm)
          SETUP_OLM=true
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

if [ -z "$SETUP_ALL" ] && [ -z "$SETUP_KAFKA" ] && [ -z "$SETUP_INVENTORY" ] && [ -z "$SETUP_ELASTICSEARCH" ] && [ -z "$SETUP_XJOIN_OPERATOR" ] && [ -z "$SETUP_PULL_SECRET" ] && [ -z "$SETUP_OLM" ]; then
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
PULL_SECRET=cloudservices-pull-secret
if [ "$SETUP_PULL_SECRET" = true ] || [ "$SETUP_ALL" = true ]; then
  echo "Setting up pull secret"
  kubectl create secret generic "$PULL_SECRET" --from-file=.dockerconfigjson="$HOME/.docker/config.json" --type=kubernetes.io/dockerconfigjson -n "$PROJECT_NAME"
  kubectl patch serviceaccount default -p "{\"imagePullSecrets\": [{\"name\": \"$PULL_SECRET\"}]}" -n "$PROJECT_NAME"
fi

#elasticsearch
if [ "$SETUP_ELASTICSEARCH" = true ] || [ "$SETUP_ALL" = true ]; then
  echo "Setting up Elasticsearch"
  kubectl apply -f elasticsearch.secret.clowder.yml -n "$PROJECT_NAME"

#  kubectl create -n "$PROJECT_NAME" -f elasticsearch.yml
#  sleep 60
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
  kubectl apply -f dev/xjoin.configmap.clowder.yaml -n "$PROJECT_NAME"
  kubectl apply -f dev/xjoin.role.clowder.yaml -n "$PROJECT_NAME"
  kubectl apply -f dev/xjoin.rolebinding.clowder.yaml -n "$PROJECT_NAME"

  if [ "$SETUP_DEV" = true ]; then
    make install
  fi
fi

# OLM
if [ "$SETUP_OLM" = true ] || [ "$SETUP_ALL" = true ]; then
  echo "Installing OLM"
  CURRENT_DIR=$(pwd)
  mkdir /tmp/olm
  cd /tmp/olm || exit 1
  curl -L https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.19.1/install.sh -o install.sh
  chmod +x install.sh
  ./install.sh v0.19.1
  rm -r /tmp/olm
  cd "$CURRENT_DIR" || exit 1
fi

#forward-ports
if [ "$FORWARD_PORTS" = true ] || [ "$SETUP_ALL" = true ]; then
  ./dev/forward-ports-clowder.sh "$PROJECT_NAME"
fi

echo "Done."
