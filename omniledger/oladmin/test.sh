#!/usr/bin/env bash

DBG_TEST=2
# Debug-level for app
DBG_APP=2
DBG_SRV=2
NBR_SERVERS=8
NBR_SERVERS_GROUP=8
export DEBUG_COLOR=true
. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

main(){
    startTest
    buildConode github.com/dedis/cothority/omniledger
	#test Create
    test NewEpoch
	#test Status

	#test Evolve
    stopTest
}

testCreate(){
    runCoBG 1 2 3 4 5 6 7 8

    testFail run create
	testFail run create -shards
	testFail run create -shards 0
	testFail run create -shards 1
	testFail run create -shards 1 -epoch
	testFail run create -shards 1 -epoch 500
	testOK run create -shards 2 -epoch 500 public.toml
	rm -f key*.cfg ol*.cfg
}

testEvolve(){
	runCoBG 1 2
	testOK run create -shards 2 -epoch 10 roster.toml
	testFail run evolve 
	testFail run evolve ol.cfg
	testFail run evolve ol.cfg key.cfg
	testOK run evolve ol.cfg key.cfg newroster.toml
	rm -f key*.cfg ol*.cfg
}

testNewEpoch(){
	runCoBG 1 2 3 4 5 6 7 8
	testOK run create -shards 2 -epoch 500 public.toml

	ol=(ol*.cfg )
	echo $ol

	key=(key*.cfg)
	echo $key

	testOK run newepoch $ol $key

	echo -----------------------------------------------------------
	echo -----------------------------------------------------------
	echo -----------------------------------------------------------
	echo -----------------------------------------------------------
	echo -----------------------------------------------------------
	echo -----------------------------------------------------------
	echo -----------------------------------------------------------
	echo -----------------------------------------------------------

	sleep 2

	testOK run newepoch $ol $key
}

testStatus() {
	runCoBG 1 2 3 4 5 6 7 8
	testOK run create -shards 2 -epoch 500 public.toml

	ol=(ol*.cfg )

	testOK run status $ol
}

run(){
    dbgRun ./$APP -d $DBG_APP $@
}

main
