#!/usr/bin/env bash

DBG_TEST=1
DBG_SRV=2
export DEBUG_LVL=2
export BC_WAIT=true

# Use 3 servers, use all of them, don't leave one down.
NBR=3
NBR_SERVERS_GROUP=$NBR

. ../../libtest.sh

# Use a local config dir.
el="./el -c ."

main(){
	build $APPDIR/../../byzcoin/bcadmin
	startTest
	buildConode go.dedis.ch/cothority/v3/eventlog

	# This must succeed before any others will work.
	run testEventLog

	stopTest
}

testEventLog(){
	##### setup phase
	rm -f *.cfg
	runCoBG 1 2 3
	runGrepSed "export BC=" "" ./bcadmin -c . create --roster public.toml --interval .5s
	eval "$SED"
	[ -z "$BC" ] && exit 1
	
	KEY=$(./el -c . key)

	./bcadmin debug counters bc*cfg key*cfg
	testOK ./bcadmin -c . darc rule -rule spawn:eventlog -identity "$KEY"
	./bcadmin debug counters bc*cfg key*cfg
	testOK ./bcadmin -c . darc rule -rule invoke:eventlog.log -identity "$KEY"

	runGrepSed "export EL=" "" $el create -sign "$KEY"
	eval "$SED"
	[ -z "$EL" ] && exit 1
	
	##### testing phase
	testOK $el log -t 'test' -c 'abc' -w 10 -sign "$KEY"
	testOK $el log -c 'def' -w 10 -sign "$KEY"
	echo ghi | testOK $el log -w 10 -sign "$KEY"
	seq 10 | testOK $el log -t seq100 -w 10 -sign "$KEY"

	testGrep "abc" $el search -t test
	testCountLines 13 $el search

	testCountLines 0 $el search -t test -from '0s ago'
	# The first form of relative date is for MacOS, the second for Linux.
	testCountLines 0 $el search -t test -from '1h ago' -to `date -v -1d +%Y-%m-%d || date -d yesterday +%Y-%m-%d`
	testCountLines 1 $el search -t test -to `date -v +1d +%Y-%m-%d || date -d tomorrow +%Y-%m-%d`
}

main
