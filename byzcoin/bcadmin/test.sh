#!/usr/bin/env bash

DBG_TEST=1
DBG_SRV=1
DBG_BCADMIN=1

NBR_SERVERS=4
NBR_SERVERS_GROUP=3

. "../../libtest.sh"

main(){
    startTest
    buildConode go.dedis.ch/cothority/v3/byzcoin go.dedis.ch/cothority/v3/byzcoin/contracts
	[ ! -x ./bcadmin ] && exit 1
    run testCoin
    run testRoster
    run testCreateStoreRead
    run testAddDarc
    run testRuleDarc
    run testAddDarcFromOtherOne
    run testAddDarcWithOwner
    run testExpression
    run testQR
    stopTest
}

testCoin(){
  rm -f config/*
  runCoBG 1 2 3
  testOK runBA create public.toml --interval .5s
  bc=config/bc*cfg
  key=config/key*cfg
  testOK runBA mint $bc $key 0000000000000000000000000000000000000000000000000000000000000000 10000
}

testRoster(){
  rm -f config/*
  runCoBG 1 2 3 4
  testOK runBA create public.toml --interval .5s
  bc=config/bc*cfg
  key=config/key*cfg
  testOK runBA show $bc
  testFail runBA roster add $bc $key co1/public.toml
  testOK runBA roster add $bc $key co4/public.toml

  # Change the block size to create a new block before verifying the roster
  testOK runBA config --blockSize 1000000 $bc $key
  testGrep 2008 runBA show $bc

  testFail runBA roster add $bc $key co4/public.toml
  testFail runBA roster del $bc $key co1/public.toml
  testOK runBA roster del $bc $key co2/public.toml
  # Change the block size to create a new block before verifying the roster
  testOK runBA config --blockSize 1000000 $bc $key
  testNGrep 2004 runBA show $bc

  testFail runBA roster del $bc $key co3/public.toml

  testFail runBA roster leader $bc $key co2/public.toml
  testFail runBA roster leader $bc $key co1/public.toml
  testOK runBA roster leader $bc $key co3/public.toml
  # Change the block size to create a new block before verifying the roster
  testOK runBA config --blockSize 1000000 $bc $key
  testGrep "Roster: tls://localhost:2006" runBA show -server 2 $bc
}

# create a ledger, and read the genesis darc.
testCreateStoreRead(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1
  testGrep "Description: \"genesis darc\"" runBA show
}

testAddDarc(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1

  testOK runBA darc add
  testOK runBA darc add -out_id ./darc_id.txt
  testOK runBA darc add
  ID=`cat ./darc_id.txt`
  testGrep "${ID:5:${#ID}-0}" runBA darc show --darc "$ID"
}

testRuleDarc(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1

  testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -desc testing -unrestricted
  ID=`cat ./darc_id.txt`
  KEY=`cat ./darc_key.txt`
  testGrep "Description: \"testing\"" runBA darc show -darc $ID
  testOK runBA darc rule -rule spawn:xxx -identity ed25519:foo -darc "$ID" -sign "$KEY"
  testGrep "spawn:xxx - \"ed25519:foo\"" runBA darc show -darc "$ID"
  testOK runBA darc rule -replace -rule spawn:xxx -identity "ed25519:foo | ed25519:oof" -darc "$ID" -sign "$KEY"
  testGrep "spawn:xxx - \"ed25519:foo | ed25519:oof\"" runBA darc show -darc "$ID"
  testOK runBA darc rule -delete -rule spawn:xxx -darc "$ID" -sign "$KEY"
  testNGrep "spawn:xxx" runBA darc show -darc "$ID"
}

testAddDarcFromOtherOne(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1

  testOK runBA darc add -out_key ./key.txt -out_id ./id.txt -unrestricted
  KEY=`cat ./key.txt`
  ID=`cat ./id.txt`
  testOK runBA darc rule -rule spawn:darc -identity "$KEY" -darc "$ID" -sign "$KEY"
  testOK runBA darc add -darc "$ID" -sign "$KEY"
}

testAddDarcWithOwner(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1

  testOK runBA key -save ./key.txt
  KEY=`cat ./key.txt`
  testOK runBA darc add -owner "$KEY" -out_id "darc_id.txt"
  ID=`cat ./darc_id.txt`
  testGrep "$KEY" runBA darc show -darc "$ID"
}

testExpression(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1

  testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
  ID=`cat ./darc_id.txt`
  KEY=`cat ./darc_key.txt`
  testOK runBA key -save ./key.txt
  KEY2=`cat ./key.txt`

  testOK runBA darc rule -rule spawn:darc -identity "$KEY | $KEY2" -darc "$ID" -sign "$KEY"
  testOK runBA darc show -darc "$ID"
  testOK runBA darc add -darc "$ID" -sign "$KEY"
  testOK runBA darc add -darc "$ID" -sign "$KEY2"

  testOK runBA darc rule -replace -rule spawn:darc -identity "$KEY & $KEY2" -darc "$ID" -sign "$KEY"
  testFail runBA darc add -darc "$ID" -sign "$KEY"
  testFail runBA darc add -darc "$ID" -sign "$KEY2"
}

runBA(){
  ./bcadmin -c config/ --debug $DBG_BCADMIN "$@"
}

testQR() {
  runCoBG 1 2 3
  runGrepSed "export BC=" "" ./"$APP" create --roster public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1

  testOK ./"$APP" qr -admin
}

main
