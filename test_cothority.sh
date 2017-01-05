#!/usr/bin/env bash

DBG_TEST=1

. $GOPATH/src/github.com/dedis/onet/app/libtest.sh

main(){
    startTest
	setupCothority
    test Build
    test Cothority
    stopTest
}

testCothority(){
    runCoBG 1 2
    cp co1/group.toml .
    tail -n 4 co2/group.toml >> group.toml
    testOK runCo 1 check -g group.toml
    tail -n 4 co3/group.toml >> group.toml
    testFail runCo 1 check -g group.toml
}

testBuild(){
    testOK dbgRun runCo 1 --help
}

main
