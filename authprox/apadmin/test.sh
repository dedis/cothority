#!/usr/bin/env bash

DBG_TEST=1
DBG_SRV=0

# Use 3 servers, use all of them, don't leave one down.
NBR=3
NBR_SERVERS_GROUP=$NBR
. ../../libtest.sh

export BC_WAIT=true

main(){
	build $APPDIR/../../byzcoin/bcadmin
	build $APPDIR/../../eventlog/el
	
	startTest
	buildConode go.dedis.ch/cothority/v3/byzcoin go.dedis.ch/cothority/v3/authprox

	# This must succeed before the others will work.
	run BCSetup

	run testAdd
	stopTest
}

BCSetup(){
	runCoBG 1 2 3
	runGrepSed "export BC=" "" ./bcadmin -c . create --roster public.toml --interval 0.5s
	eval $SED
	[ -z "$BC" ] && exit 1

	KEY=$(./el -c . key)

	testOK ./bcadmin -c . darc rule --rule spawn:authproxAdd --identity $KEY
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
	testNGrep https://NOauth.dedis.ch ./apadmin show --roster tcp.toml

}

main
