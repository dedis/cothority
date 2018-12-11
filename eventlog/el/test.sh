#!/usr/bin/env bash

DBG_TEST=2
DBG_SRV=0

# Use 3 servers, use all of them, don't leave one down.
NBR=3
NBR_SERVERS_GROUP=$NBR

. "$(go env GOPATH)/src/github.com/dedis/cothority/libtest.sh"

# Use a local config dir.
el="./el -c ."

main(){
	build $APPDIR/../../byzcoin/bcadmin
	startTest
	buildConode github.com/dedis/cothority/eventlog

	# This must succeed before any others will work.
	run testCreate
	
	run testLogging
	
	stopTest
}

# This is necessary becaues el does not remember the current signer counter from invocation
# to invocation, it always gets it from the server. And so if you call el twice
# in one block, the second one will use the same counter as the first one did.
waitBlock(){
	sleep .5
}

testLogging(){
	runCoBG 1 2 3
	testOK $el log -t test -c 'abc'
	waitBlock
	testOK $el log -c 'def'
	waitBlock
	waitBlock
	echo ghi | testOK $el log
	waitBlock
	seq 100 | testOK $el log -t seq100

	# Wait two block intervals to be sure they are all committed
	waitBlock
	waitBlock

	testGrep "abc" $el search -t test
	testCountLines 103 $el search

	testCountLines 0 $el search -t test -from '0s ago'
	testCountLines 0 $el search -t test -from '1h ago' -to `date -v -1d +%Y-%m-%d`
	testCountLines 1 $el search -t test -to `date -v +1d +%Y-%m-%d`
}

testCreate(){
	runGrepSed "export PRIVATE_KEY=" "" ./el key
	eval $SED
	[ -z "$PRIVATE_KEY" ] && exit 1
	ID=`awk '/^Identity: / { print $2}' < $RUNOUT`
	[ -z "$ID" ] && exit 1

	runCoBG 1 2 3
	runGrepSed "export BC=" "" ./bcadmin -c . create --roster public.toml --interval 0.5s
	eval $SED
	[ -z "$BC" ] && exit 1
	
	testOK ./bcadmin -c . add spawn:eventlog -identity $ID
	testOK ./bcadmin -c . add invoke:eventlog -identity $ID
	testGrep $ID ./bcadmin -c . show

	runGrepSed "export EL=" "" $el create
	eval $SED
	[ -z "$EL" ] && exit 1
	
	# We do not want cleanup to remove the db between each test.
	export KEEP_DB=true
}

main
