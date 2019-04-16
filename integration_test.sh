#!/usr/bin/env bash

for t in $( find . -name test.sh )
do
	d=`dirname $t`
	echo -e "\n** Running integration-test $t"
	( cd $d; ./test.sh -b ) || exit 1
done
