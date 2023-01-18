#!/bin/bash

go version

# including number of comments as it was used by clowder.
while read line; do
    if [ ${#line} -ge 100 ]; then
        echo "Commit messages are limited to 100 characters."
        echo "The following commit message has ${#line} characters."
        echo "${line}"
        exit 1
    fi
done <<< "$(git log --pretty=format:%s $(git merge-base master HEAD)..HEAD)"

set -exv

IMAGE_TAG=`cat go.mod go.sum Dockerfile | sha256sum  | head -c 7`
TEST_IMAGE="xjoin-operator-unit-"$IMAGE_TAG

# check container engine type
podman_state=`systemctl show --property ActiveState podman`
docker_state=`systemctl show --property ActiveState docker`

if grep -iqw "active" <<< $podman_state; then
    CONTAINER_ENGINE='podman'
elif grep -iqw "active" <<< $docker_state; then
    CONTAINER_ENGINE='docker'
else
    echo "No container engine running"
fi

echo "Active container engine: $CONTAINER_ENGINE"

$CONTAINER_ENGINE build -t $TEST_IMAGE -f Dockerfile.4unittest . 
$CONTAINER_ENGINE run -i -u0 $TEST_IMAGE bash -c "make generic-test"
TEST_RESULT=$?

if [[ $TEST_RESULT -ne 0 ]]; then
    echo "Test encountered problems"
    exit $TEST_RESULT
else
    echo "Unit tests SUCCESSFUL"
fi

exit $TEST_RESULT
