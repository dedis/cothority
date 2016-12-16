#!/usr/bin/env bash

DBG_SHOW=1
# Debug-level for server
DBG_SRV=0
# For easier debugging
STATICDIR=test
NBR_CLIENTS=3
NBR_SERVERS=3

. $GOPATH/src/github.com/dedis/onet/app/tesht/libtest.sh
. $GOPATH/src/github.com/dedis/onet/app/tesht/cothority.sh

main(){
    startTest
    buildApp
    test Build
    test Cothority
    test Link
    stopTest
}

testLink(){
	runCoBG 1
	runCoBG 2
	testOK time
}

testBuild(){
    testOK dbgRun ./cothority --help
    testOK dbgRun ./pop --help
}

buildApp(){
	makeTestDir
    echo "Creating directories"
    for n in $(seq $NBR_CLIENTS); do
        cl=cl$n
        rm -f $cl/*
        mkdir -p $cl
    done
}

if [ "$1" -a "$STATICDIR" ]; then
    rm -f $STATICDIR/{cothority,pop}
fi

main
