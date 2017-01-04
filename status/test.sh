#!/usr/bin/env bash

DBG_TEST=1
DBG_APP=2
. $GOPATH/src/github.com/dedis/onet/app/libtest.sh

main(){
    startTest
    buildCothority
    test Build
    test Network
    stopTest
}

testNetwork(){
	runCoBG 1 2
    testOut "Running network"
    testGrep "Available_Services" runCl -g group.toml
    testGrep "Available_Services" runCl -g group.toml
}

testBuild(){
    testOK runCl --help
    testOK runCo 1 --help
}

runCl(){
    dbgRun ./status -d $DBG_APP $@
}

main
