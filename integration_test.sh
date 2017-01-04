#!/usr/bin/env bash

for t in $( find . -name test.sh ); do
	echo "Running integration-test $t"
	( cd $( dirname $t ); ./$( basename $t ) ) || exit 1
done