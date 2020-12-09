#!/bin/bash

oc whoami || exit 1

oc delete project xjoin-operator-project &
oc delete project kafka &
oc delete project elastic-system &

wait
pkill -f "oc port-forward"
echo "Done"
