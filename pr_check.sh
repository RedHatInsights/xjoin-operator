#!/bin/bash

# --------------------------------------------
# Options that must be configured by app owner
# --------------------------------------------
APP_NAME="xjoin"  # name of app-sre "application" folder this component lives in
COMPONENT_NAME="xjoin-operator"  # name of app-sre "resourceTemplate" in deploy.yaml for this component
IMAGE="quay.io/cloudservices/xjoin-operator"  # image location on quay

IQE_PLUGINS=pr_check"xjoin"  # name of the IQE plugin for this app.
IQE_MARKER_EXPRESSION="smoke"  # This is the value passed to pytest -m
IQE_FILTER_EXPRESSION=""  # This is the value passed to pytest -k
IQE_CJI_TIMEOUT="30m"  # This is the time to wait for smoke test to complete or fail

DOCKERFILE="Dockerfile.unittest"

# the below part works.
ARTIFACTS_DIR=$WORKSPACE/artifacts

mkdir -p $ARTIFACTS_DIR
cat << EOF > $ARTIFACTS_DIR/junit-dummy.xml
<testsuite tests="1">
    <testcase classname="dummy" name="dummytest"/>
</testsuite>
EOF
