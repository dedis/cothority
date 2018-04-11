#!/usr/bin/env bash

DBG_TEST=2
# Debug-level for app
DBG_APP=2
DBG_SRV=2

. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

main(){
    startTest
    buildConode
    test CreateStoreRead
    stopTest
}

testCreateStoreRead(){
    runCoBG 1 2
    runGrepSed ID "s/.* //" ./$APP create public.toml
    ID=$SED
    echo ID is $ID
    testOK runSicpa set public.toml $ID one two
    testOK runSicpa get public.toml $ID one
    testFail runSicpa get public.toml $ID two
}

runSicpa(){
    dbgRun ./$APP -d $DBG_APP $@
}

main
