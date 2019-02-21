#!/usr/bin/env bash

DBG_TEST=1
DBG_SRV=2
DBG_BA=2
DBG_PH=2

NBR_SERVERS=4
NBR_SERVERS_GROUP=3

ZERO_KEY=0000000000000000000000000000000000000000000000000000000000000000

. "$(go env GOPATH)/src/github.com/dedis/cothority/libtest.sh"

main(){
    startTest
    buildConode github.com/dedis/cothority/byzcoin github.com/dedis/cothority/byzcoin/contracts github.com/dedis/cothority/personhood
    build $APPDIR/../../byzcoin/bcadmin
    run testSpawner
    run testWipe
    run testRegister
    stopTest
}

testSpawner(){
  rm -f config/*
  runCoBG 1 2 3
  testOK runBA create public.toml --interval .5s
  testOK runPH spawner -darc 123 -coin 234 -credential 345 -party 456 config/bc*cfg config/key*cfg
  testFileGrep "costDarc to 123" ${COLOG}1.log
  testFileGrep "costCoin to 234" ${COLOG}1.log
  testFileGrep "costCredential to 345" ${COLOG}1.log
  testFileGrep "costParty to 456" ${COLOG}1.log
}

testWipe(){
  rm -f config/*
  runCoBG 1 2 3
  testOK runBA create public.toml --interval .5s
  testOK runPH wipe config/bc*cfg
  testFileGrep "Wiping party cache" ${COLOG}1.log
  testFileGrep "Wiping party cache" ${COLOG}2.log
  testFileGrep "Wiping party cache" ${COLOG}3.log
}

testRegister(){
  rm -f config/*
  runCoBG 1 2 3
  testOK runBA create public.toml --interval .5s
  bc=config/bc*cfg
  key=config/key*cfg
  testFail runPH show $bc $ZERO_KEY
  pub="public_ed25519=$ZERO_KEY"
  alias="alias=foo"
  testOK runPH register config/bc*cfg config/key*cfg "https://pop.dedis.ch/qrcode/unregistered-1?$pub&$alias"
  testGrep "" runPH show config/bc*cfg ad85845de51f6609587a545311d845cb803df2a02ab868930fd07e85b05fb4bc
  testReGrep "ed25519: 0000000000000000000000000000000000000000000000000000000000000000"
  testReGrep "darcID: 1ca978335adb086275ac35e8b338831b6a2c38202e95a7d5e0541d8d074aa9c9"
  testReGrep "coinIID: ff2eac64567ddca91e64344cfb896f92fa375f6850ddb28b829594cf38b92449"
}

runBA(){
  ./bcadmin -c config/ --debug $DBG_BA "$@"
}

runPH(){
  ./phapp --debug $DBG_PH "$@"
}
main
