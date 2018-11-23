#!/usr/bin/env bash

DBG_TEST=1
DBG_SRV=0
DBG_BA=2

NBR_SERVERS=4
NBR_SERVERS_GROUP=3

. "$(go env GOPATH)/src/github.com/dedis/cothority/libtest.sh"

main(){
  startTest
  buildConode github.com/dedis/cothority/byzcoin github.com/dedis/cothority/byzcoin/contracts
  rm -rf config
  run testCoin
  run testRoster
  run testShow
  run testCreateStoreRead
  run testAddDarc
  run testRuleDarc
  run testAddDarcFromOtherOne
  run testAddDarcWithOwner
  run testExpression
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
  testGrep "Roster: tls://localhost:2006" runBA show $bc
}

testShow(){
  rm -f config/*
  runCoBG 1 2 3
  testOK runBA create public.toml --interval .5s
  testOK runBA show config/bc*cfg
}

testCreateStoreRead(){
  rm -f config/*
  runCoBG 1 2 3
  runGrepSed "export BC=" "" runBA create public.toml --interval .5s
  eval $SED
  [ -z "$BC" ] && exit 1
  testOK runBA add spawn:xxx -identity ed25519:foo
  testGrep "spawn:xxx - \"ed25519:foo\"" runBA show
  # Should not allow overwrite on rule without replace.
  testFail runBA add spawn:xxx -identity "& ed25519:foo ed25519:bar"
  testOK runBA add spawn:xxx -replace -identity "& ed25519:foo ed25519:bar"
  testGrep "spawn:xxx - \"& ed25519:foo ed25519:bar\"" runBA show
  # Do not allow both, neither.
  testFail runBA add spawn:xxx -identity id -expression exp
  testFail runBA add spawn:xxx
}

testAddDarc(){
  runCoBG 1 2 3
  runGrepSed "export BC=" "" ./"$APP" create public.toml --interval .5s
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
  runGrepSed "export BC=" "" ./"$APP" create public.toml --interval .5s
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
  runGrepSed "export BC=" "" ./"$APP" create public.toml --interval .5s
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
  runGrepSed "export BC=" "" ./"$APP" create public.toml --interval .5s
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
  runGrepSed "export BC=" "" ./"$APP" create public.toml --interval .5s
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

runBA(){
  ./bcadmin -c config/ --debug $DBG_BA "$@"
}

main
