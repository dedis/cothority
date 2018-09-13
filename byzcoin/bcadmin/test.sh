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
    runGrepSed "export BC=" "" ./"$APP" create --roster public.toml --interval .5s
	eval $SED
	[ -z "$BC" ] && exit 1
    testOK ./"$APP" add spawn:xxx -identity ed25519:foo
	testGrep "ed25519:foo" ./"$APP" show
}

main
