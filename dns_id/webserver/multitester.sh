#!/usr/bin/env bash

NUM_OF_TESTS=100

for ((i=1;i<=$NUM_OF_TESTS;i++)); do
	echo $i
	x=`go test -run TestSkipchainSwitchNoConc | grep PASS`
	if [ "$x" == "" ]; then
		echo "ERROR"
		echo "______________________________________________________"
		echo "______________________________________________________"
	fi

done
