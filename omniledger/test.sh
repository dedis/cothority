#!/usr/bin/env bash

DBG_TEST=2
# Debug-level for app
DBG_APP=2
DBG_SRV=2

. $(go env GOPATH)/src/gopkg.in/dedis/onet.v2/app/libtest.sh

main(){
    startTest
    buildConode github.com/dedis/student_18_omniledger/omniledger/service
    test CreateStoreRead
    stopTest
}

testCreateStoreRead(){
    pair=$(./"$APP" keypair)
    pk=$(echo "$pair" | grep Public | sed 's/Public: //')
    # sk=$(echo "$pair" | grep Private | sed 's/Private: //')
    runCoBG 1 2 3
    runGrepSed ID "s/.* //" ./"$APP" create public.toml "$pk"
    ID=$SED
    echo ID is "$ID"
    testOK runOmni set public.toml "$ID" one two
    testOK runOmni get public.toml "$ID" one
    testFail runOmni get public.toml "$ID" two
}

runOmni(){
    dbgRun ./"$APP" -d $DBG_APP "$@"
}

main
