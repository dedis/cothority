#!/usr/bin/env bash

DBG_TEST=1
# Debug-level for app
DBG_APP=2

. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

main(){
    startTest
    buildConode
	test Create
    test Evolve
    test NewEpoch
    stopTest
}

testCreate(){
    runCoBG 1 2
    testFail run create
	testFail run create -shards
	testFail run create -shards 0
	testFail run create -shards 1
	testFail run create -shards 1 -epoch
	testFail run create -shards 1 -epoch 1
	testOK run create -shards 1 -epoch 1 roster.toml
}

testEvolve(){
	runCoBG 1 2
	testOK run create -shards 2 -epoch 10 roster.toml
	testFail run evolve 
	testFail run evolve ol.cfg
	testFail run evolve ol.cfg key.cfg
	testOK run evolve ol.cfg key.cfg newroster.toml
}

testNewEpoch(){
	runCoBG 1 2
	testOK run create -shards 2 -epoch 10 roster.toml
	testFail run newepoch 
	testFail run newepoch ol.cfg
	testOK run newepoch ol.cfg key.cfg
}

run(){
    dbgRun ./$APP -d $DBG_APP $@
}

main
