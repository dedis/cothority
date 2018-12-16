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

testNewEpoch(){
	runCoBG 1 2 3 4 5 6 7 8
	testOK run create -shards 2 -epoch 500 public.toml

	ol=(ol*.cfg )
	echo $ol

	key=(key*.cfg)
	echo $key

	testFail run newepoch
	testFail run newepoch $ol
	testFail run newepoch $key $ol

	testOK run newepoch $ol $key
	sleep 2
	testOK run newepoch $ol $key
}

testStatus() {
	runCoBG 1 2 3 4 5 6 7 8
	testOK run create -shards 2 -epoch 500 public.toml

	ol=(ol*.cfg )

	testFail run status
	testOK run status $ol
}

run(){
    dbgRun ./$APP -d $DBG_APP $@
}

main
