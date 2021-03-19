#!/bin/bash

USAGE="USAGE: ./install.operator.with.operator.sdk.sh <QUAY_USERNAME> <QUAY_PASSWORD>"

if [ -z "$1" ]
then
  echo "Must provide QUAY_USERNAME"
  echo "$USAGE"
  exit 1
fi

QUAY_USERNAME=$1

make docker-build -e QUAY_NAMESPACE=$QUAY_USERNAME
docker push -a quay.io/$QUAY_USERNAME/xjoin-operator
make bundle-build -e BUNDLE_IMAGE=xjoin-operator-bundle BUNDLE_IMAGE_TAG=latest -e QUAY_NAMESPACE=$QUAY_USERNAME
docker tag xjoin-operator-bundle:latest quay.io/$QUAY_USERNAME/xjoin-operator-bundle:latest
docker push quay.io/$QUAY_USERNAME/xjoin-operator-bundle:latest
oc create ns xjoin-operator-olm
oc apply -f dev/elasticsearch.secret.yml
oc apply -f dev/inventory-db.secret.yml
operator-sdk run bundle quay.io/$QUAY_USERNAME/xjoin-operator-bundle:latest --namespace xjoin-operator-olm --install-mode OwnNamespace --verbose
