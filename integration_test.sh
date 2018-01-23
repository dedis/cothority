#!/usr/bin/env bash

# Many integration tests are currently broken. When they are all fixed,
# this is how to run them.
#for t in $( find . -name test.sh ); do

# For now, run the ones that are fixed.
for t in conode/test.sh scmgr/test.sh status/test.sh cosi/test.sh
do
	echo "Running integration-test $t"
	( cd $( dirname $t ); ./$( basename $t ) ) || exit 1
done
