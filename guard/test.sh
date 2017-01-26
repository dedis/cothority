#!/usr/bin/env bash

DBG_TEST=1
DBG_APP=2

. $GOPATH/src/github.com/dedis/onet/app/libtest.sh

main(){
    startTest
    buildConode
    test Build
    test Network
    stopTest
}

testNetwork(){
	runCoBG 1 2
    testOut "Running Guard"
    testOK runCl su public.toml
    testOK runCl s evan dadada Hello
    testFail runCl r evan dadadas
    testOK runCl r evan dadada
    testNGrep "Hello" runCl r evan dadadas
    testGrep "Hello" runCl r evan dadada

}

testBuild(){
    testOK runCl --help
    testOK runCo 1 --help
}

runCl(){
    dbgRun ./guard -d $DBG_APP $@
}

main
