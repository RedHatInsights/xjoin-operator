#!/bin/bash

# use the following make rule to execute unit tests
make generic-test
result=$?
set -e

# If your unit tests store junit xml results, you should store them in a file matching format `artifacts/junit-*.xml`
# If you have no junit file, use the below code to create a 'sample' result file so Jenkins will not fail
mkdir -p $ARTIFACTS_DIR
cat << EOF > $ARTIFACTS_DIR/junit-sample.xml
<testsuite tests="1">
    <testcase classname="sample" name="dummytest"/>
</testsuite>
EOF

if [ $result -ne 0 ]; then
  echo '====================================='
  echo '====  ✖ ERROR: UNIT TEST FAILED  ===='
  echo '====================================='
  exit 1
fi
