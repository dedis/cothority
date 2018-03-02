#!/usr/bin/env bash

export DBG_TEST=2
# Debug-level for app
export DBG_APP=2

. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

main(){
    startTest
    buildConode
    addr1=$( grep Address co1/public.toml | sed -e "s/.*\"\(.*\)\"/\1/")
    test Status
    stopTest
}

testStatus(){
  runCoBG 1
  testOK runAuth status co1/public.toml
  testOK runAuth status $addr1
}

testBuild(){
    testOK dbgRun runAuth --help
}

runAuth(){
    dbgRun ./$APP -d $DBG_APP "$@"
}

main
