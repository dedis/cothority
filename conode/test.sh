#!/usr/bin/env bash

DBG_TEST=1

. "$(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh"

main(){
    startTest
    setupConode
    test Build
    test Conode
    stopTest
}

testConode(){
    runCoBG 1 2
    cp co1/public.toml .
    cat co2/public.toml >> public.toml
    testOK runCo 1 check -g public.toml
    cat co3/public.toml >> public.toml
    testFail runCo 1 check -g public.toml
}

testBuild(){
    testOK dbgRun runCo 1 --help
}

main
