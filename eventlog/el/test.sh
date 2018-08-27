#!/usr/bin/env bash

DBG_TEST=2
DBG_SRV=0

. "$(go env GOPATH)/src/github.com/dedis/cothority/libtest.sh"

main(){
	build $APPDIR/../../omniledger/ol
    startTest
    buildConode github.com/dedis/cothority/eventlog

	# This must succeed before any others will work.
    run testCreate
	
	run testLogging
	
    stopTest
}

testLogging(){
	runCoBG 1 2 3
	testOK ./el log -t test -c 'abc'
	testOK ./el log -c 'def'
	echo ghi | testOK ./el log 
	seq 100 | testOK ./el log -t seq100

	# two block intervals (2 * 0.5s)
	sleep 1

	testGrep "abc" ./el search -t test
	testCountLines 103 ./el search

	testCountLines 0 ./el search -t test -from '0s ago'
	testCountLines 0 ./el search -t test -from '1h ago' -to `date -v -1d +%Y-%m-%d`
	testCountLines 1 ./el search -t test -to `date -v +1d +%Y-%m-%d`
}

testCreate(){
	runGrepSed "export PRIVATE_KEY=" "" ./el create --keys
	eval $SED
	[ -z "$PRIVATE_KEY" ] && exit 1
	ID=`awk '/^Identity: / { print $2}' < $RUNOUT`
	[ -z "$ID" ] && exit 1
	
	runCoBG 1 2 3
    runGrepSed "export OL=" "" ./ol create --roster public.toml --interval 0.5s
	eval $SED
	[ -z "$OL" ] && exit 1
	
    testOK ./ol add spawn:eventlog -identity $ID
    testOK ./ol add invoke:eventlog -identity $ID
	testGrep $ID ./ol show
	
	runGrepSed "export EL=" "" ./el create
	eval $SED
	[ -z "$EL" ] && exit 1
	
	# We do not want cleanup to remove the db between each test.
	export KEEP_DB=true
}

main
