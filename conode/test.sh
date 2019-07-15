#!/usr/bin/env bash

DBG_TEST=1
DBG_SRV=0

. ../libtest.sh

main(){
    startTest
    setupConode
    run testBuild
    run testConode
    run testDatabase
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

testDatabase(){
    runCoBG 1

    # rename database file
    hexPK=$(cat co1/public.toml  | grep -m 1 'Public = '  | cut -d = -f 2 - | tr -d ' "')
    shaPK=$(ls -t "$CONODE_SERVICE_PATH" | head -n 1 | sed 's/.db$//g')
    testOK mv "$CONODE_SERVICE_PATH/$shaPK.db" "$CONODE_SERVICE_PATH/$hexPK.db"

    # run conode again and the file should be renamed
    pkill conode
    rm -f "$COLOG"1.log.dead
    runCoBG 1
    testFail [ -f "$CONODE_SERVICE_PATH/$hexPK.db" ]
    testOK [ -f "$CONODE_SERVICE_PATH/$shaPK.db" ]
}

testBuild(){
    testOK dbgRun runCo 1 --help
}

main
