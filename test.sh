#!/usr/bin/env bash

DBG_SHOW=2
# Debug-level for server
DBG_SRV=0
# For easier debugging
#STATICDIR=test

. ./libcothority/cothority.sh

main(){
    startTest
	appBuild
	cothorityBuild
    test Build
    test Cothorityd
    stopTest
}

testCothorityd(){
    runCoCfg 1
    runCoCfg 2
    runCoCfg 3
    runCoBG 1
    runCoBG 2
    sleep 1
    cp co1/group.toml .
    tail -n 4 co2/group.toml >> group.toml
    testOK runCo 1 check -g group.toml
    tail -n 4 co3/group.toml >> group.toml
    testFail runCo 1 check -g group.toml
}

testBuild(){
    testOK dbgRun ./cothority --help
}

main
