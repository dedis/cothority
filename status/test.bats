#!/usr/bin/env bats

DBG_TEST=1
DBG_APP=2

setup() {
    . $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh
    startTest
    buildConode
}

teardown() {
    stopTest
}

runCl(){
    dbgRun ./status -d $DBG_APP $@
}

@test "network" {
    runCoBG 1 2
    testOut "Running network"
    testGrep "Available_Services" runCl -g public.toml
    testGrep "Available_Services" runCl -g public.toml
}

@test "build" {
    runCl --help
    runCo 1 --help
}
