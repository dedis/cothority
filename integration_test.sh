#!/usr/bin/env bash

# Many integration tests are currently broken. When they are all fixed,
# this is how to run them.
#for t in $( find . -name test.sh ); do

# For now, run the ones that are fixed.
for t in conode scmgr status cosi pop cisc
do
	echo -e "\n** Running integration-test $t"
	( cd $t; ./test.sh ) || exit 1
done
