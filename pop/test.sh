#!/usr/bin/env bash

DBG_TEST=1
DBG_APP=3
NBR_CLIENTS=3
NBR_SERVERS=3

. $GOPATH/src/gopkg.in/dedis/onet.v1/app/libtest.sh

main(){
    startTest
    buildConode
    echo "Creating directories"
    for n in $(seq $NBR_CLIENTS); do
        cl=cl$n
        rm -f $cl/*
        mkdir -p $cl
    done
    addr1=127.0.0.1:2002
    addr2=127.0.0.1:2004
    addr3=127.0.0.1:2006

    test Build
    test Check
    stopTest
}

testCheck(){
	runCoBG 1 2 3
	cat co*/public.toml > check.toml
	testOK dbgRun ./pop -d $DBG_APP check check.toml
}

testBuild(){
    testOK dbgRun ./conode --help
    testOK dbgRun ./pop --help
}

runCl(){
    local CFG=cl$1
    shift
    dbgRun ./pop -d $DBG_APP -c $CFG $@
}

runDbgCl(){
	local DBG=$1
	local CFG=cl$2
	shift 2
	./pop -d $DBG -c $CFG $@
}

main
