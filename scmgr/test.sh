#!/usr/bin/env bash

DBG_TEST=1
# Debug-level for app
DBG_APP=2

. $GOPATH/src/github.com/dedis/onet/app/libtest.sh

main(){
    startTest
    buildConode
	test Count
	test Time
    stopTest
}

testCount(){
       runCoBG 1 2
       testFail runTmpl counter
       testOK runTmpl counter public.toml
       testGrep ": 0" runTmpl counter public.toml
       runTmpl time public.toml
       testGrep ": 1" runTmpl counter public.toml
}

testTime(){
       runCoBG 1 2
       testFail runTmpl time
       testOK runTmpl time public.toml
       testGrep Time runTmpl time public.toml
}

testBuild(){
    testOK dbgRun runTmpl --help
}

runTmpl(){
    dbgRun ./$APP -d $DBG_APP $@
}

main
