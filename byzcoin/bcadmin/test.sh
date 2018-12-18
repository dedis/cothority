#!/usr/bin/env bash

DBG_TEST=2
DBG_SRV=0

NBR_SERVERS=3
NBR_SERVERS_GROUP=3

. "$(go env GOPATH)/src/github.com/dedis/cothority/libtest.sh"

main(){
    startTest
    buildConode github.com/dedis/cothority/byzcoin
    run testCreateStoreRead
    run testAddDarc
    run testRuleDarc
    run testAddDarcFromOtherOne
    run testAddDarcWithOwner
    run testExpression
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

testAddDarc(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" ./"$APP" create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1

  testOK ./"$APP" darc add
  testOK ./"$APP" darc add -out_id ./darc_id.txt
  testOK ./"$APP" darc add
  ID=`cat ./darc_id.txt`
  testGrep "${ID:5:${#ID}-0}" ./"$APP" darc show --darc "$ID"
}

testRuleDarc(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" ./"$APP" create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1

  testOK ./"$APP" darc add -out_id ./darc_id.txt -out_key ./darc_key.txt
  ID=`cat ./darc_id.txt`
  KEY=`cat ./darc_key.txt`
  testOK ./"$APP" darc rule -rule spawn:xxx -identity ed25519:foo -darc "$ID" -sign "$KEY"
  testGrep "spawn:xxx - \"ed25519:foo\"" ./"$APP" darc show -darc "$ID"
  testOK ./"$APP" darc rule -replace -rule spawn:xxx -identity "ed25519:foo | ed25519:oof" -darc "$ID" -sign "$KEY"
  testGrep "spawn:xxx - \"ed25519:foo | ed25519:oof\"" ./"$APP" darc show -darc "$ID"
  testOK ./"$APP" darc rule -delete -rule spawn:xxx -darc "$ID" -sign "$KEY"
  testNGrep "spawn:xxx" ./"$APP" darc show -darc "$ID"
}

testAddDarcFromOtherOne(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" ./"$APP" create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1

  testOK ./"$APP" darc add -out_key ./key.txt -out_id ./id.txt
  KEY=`cat ./key.txt`
  ID=`cat ./id.txt`
  testOK ./"$APP" darc rule -rule spawn:darc -identity "$KEY" -darc "$ID" -sign "$KEY"
  testOK ./"$APP" darc add -darc "$ID" -sign "$KEY"
}

testAddDarcWithOwner(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" ./"$APP" create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1

  testOK ./"$APP" key -save ./key.txt
  KEY=`cat ./key.txt`
  testOK ./"$APP" darc add -owner "$KEY" -out_id "darc_id.txt"
  ID=`cat ./darc_id.txt`
  testGrep "$KEY" ./"$APP" darc show -darc "$ID"
}

testExpression(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" ./"$APP" create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1

  testOK ./"$APP" darc add -out_id ./darc_id.txt -out_key ./darc_key.txt
  ID=`cat ./darc_id.txt`
  KEY=`cat ./darc_key.txt`
  testOK ./"$APP" key -save ./key.txt
  KEY2=`cat ./key.txt`

  testOK ./"$APP" darc rule -rule spawn:darc -identity "$KEY | $KEY2" -darc "$ID" -sign "$KEY"
  testOK ./"$APP" darc show -darc "$ID"
  testOK ./"$APP" darc add -darc "$ID" -sign "$KEY"
  testOK ./"$APP" darc add -darc "$ID" -sign "$KEY2"

  testOK ./"$APP" darc rule -replace -rule spawn:darc -identity "$KEY & $KEY2" -darc "$ID" -sign "$KEY"
  testFail ./"$APP" darc add -darc "$ID" -sign "$KEY"
  testFail ./"$APP" darc add -darc "$ID" -sign "$KEY2"
}

main
