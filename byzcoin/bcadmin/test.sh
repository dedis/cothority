#!/usr/bin/env bash

DBG_TEST=1
DBG_SRV=0

. "$(go env GOPATH)/src/github.com/dedis/cothority/libtest.sh"

main(){
    startTest
    buildConode github.com/dedis/cothority/byzcoin
    run testCreateStoreRead
    stopTest
}

testCreateStoreRead(){
	runCoBG 1 2 3
    runGrepSed "export BC=" "" ./bcadmin create --roster public.toml --interval .5s
	eval $SED
	[ -z "$BC" ] && exit 1
    testOK ./bcadmin add spawn:xxx -identity ed25519:foo
	testGrep "spawn:xxx - \"ed25519:foo\"" ./bcadmin show
	# Should not allow overwrite on rule without replace.
    testFail ./bcadmin add spawn:xxx -identity "& ed25519:foo ed25519:bar"
    testOK ./bcadmin add spawn:xxx -replace -identity "& ed25519:foo ed25519:bar"
	testGrep "spawn:xxx - \"& ed25519:foo ed25519:bar\"" ./bcadmin show
	# Do not allow both, neither.
    testFail ./bcadmin add spawn:xxx -identity id -expression exp
    testFail ./bcadmin add spawn:xxx
}

main
