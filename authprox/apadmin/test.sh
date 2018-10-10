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
	testOK ./apadmin add --roster public.toml -issuer https://oauth.dedis.ch
	testGrep https://oauth.dedis.ch ./apadmin show --roster public.toml

	# Check that secrets are not sent unencrypted.
	cat > tcp.toml << %%
[[servers]]
  Address = "tcp://server.example.com:7002"
  Suite = "Ed25519"
  Public = "805a20360392d3fd5170ad5be862ecaf76b40eeaa0642aa3f96557df563bef8d"
  Description = "A fake server to test refusing sending secrets over TCP."
%%
	testFail ./apadmin add --roster tcp.toml -issuer https://NOauth.dedis.ch/
	testNGrep https://NOauth.dedis.ch ./apadmin show
}

main
