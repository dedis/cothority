#!/usr/bin/env bash

for t in $( find . -name test.sh )
do
	d=`dirname $t`
	echo -e "\n** Running integration-test $t"
	( cd $d; ./test.sh ) || exit 1
done
