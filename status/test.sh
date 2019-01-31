#!/usr/bin/env bash

DBG_TEST=1
DBG_APP=2

. ../libtest.sh

main(){
    startTest
    buildConode go.dedis.ch/cothority/v3/status/service
    run testBuild
    run testNetwork
    stopTest
}

testNetwork(){
    runCoBG 1 2
    testOut "Running network"
    testGrep "Available_Services" runCl -g public.toml
    testGrep "Available_Services" runCl -g public.toml
    testGrep "Available_Services" runCl --host localhost:2002
    testGrep "Available_Services" runCl --host tls://localhost:2002
}

testBuild(){
    testOK runCl --help
    testOK runCo 1 --help
}

runCl(){
    dbgRun ./status -d $DBG_APP $@
}

main
