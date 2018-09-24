#!/usr/bin/env bash

DBG_TEST=2
DBG_SRV=0

. "$(go env GOPATH)/src/github.com/dedis/cothority/libtest.sh"

main(){
	build $APPDIR/../../byzcoin/bcadmin
    startTest
    buildConode github.com/dedis/cothority/byzcoin github.com/dedis/cothority/authprox

	# This must succeed before the others will work.
	run BCSetup
	
    run testAdd
    stopTest
}

BCSetup(){
	runGrepSed "export PRIVATE_KEY=" "" ./bcadmin keys
	eval $SED
	[ -z "$PRIVATE_KEY" ] && exit 1
	ID=`awk '/^Identity: / { print $2}' < $RUNOUT`
	[ -z "$ID" ] && exit 1
	
	runCoBG 1 2 3
	runGrepSed "export BC=" "" ./bcadmin create --roster public.toml --interval 0.5s
	eval $SED
	[ -z "$BC" ] && exit 1
	
	testOK ./bcadmin add spawn:authproxAdd -identity $ID
}

testAdd(){
	runCoBG 1 2 3
    testOK ./apadmin add --roster public.toml -issuer https://oauth.dedis.ch/
}

main
