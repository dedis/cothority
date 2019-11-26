#!/usr/bin/env bash

DBG_TEST=1
DBG_SRV=2
DBG_BA=2

NBR_SERVERS=3
NBR_SERVERS_GROUP=3

export BC_WAIT=true

. "../../libtest.sh"

main(){
  startTest
  buildConode go.dedis.ch/cothority/v3/byzcoin go.dedis.ch/cothority/v3/byzcoin/contracts
  build $APPDIR/../bcadmin
  run testMulti
  run testLoadSave
  run testCoin
  stopTest
}

testMulti(){
  rm -rf config wallet{1,2}
  runCoBG 1 2 3
  testOK runBA create public.toml --interval .5s
  bc=config/bc*cfg
  key=config/key*cfg
  testOK runWallet 1 join $bc
  runGrepSed "Public key is:" "s/.* //" runWallet 1 show
  PUB=$SED
  testOK runBA mint $bc $key $PUB 1000

  testOK runWallet 2 join $bc
  runGrepSed "Public key is:" "s/.* //" runWallet 2 show
  PUB2=$SED
  testOK runBA mint $bc $key $PUB2 1000

  testOK runWallet 2 transfer --multi 10 1 $PUB
  testGrep "Balance is: 990" runWallet 2 show
  testGrep "Balance is: 1010" runWallet 1 show

  testGrep "Only allowing" runWallet 2 transfer --multi 300 1 $PUB 2>1
  testGrep "Balance is: 790" runWallet 2 show
}

testLoadSave(){
  rm -rf config wallet{1,2}
  runCoBG 1 2 3
  testOK runBA create public.toml --interval .5s
  bc=config/bc*cfg
  testOK runWallet 1 join $bc
}

testCoin(){
  rm -rf config wallet{1,2}
  runCoBG 1 2 3
  testOK runBA create public.toml --interval .5s
  bc=config/bc*cfg
  key=config/key*cfg
  testOK runWallet 1 join $bc
  testGrep "Balance is: 0" runWallet 1 show
  runGrepSed "Public key is:" "s/.* //" runWallet 1 show
  PUB=$SED
  testOK runBA mint $bc $key $PUB 1000
  testGrep "Balance is: 1000" runWallet 1 show

  testOK runWallet 2 join $bc
  runGrepSed "Public key is:" "s/.* //" runWallet 2 show
  PUB2=$SED
  testGrep "Balance is: 0" runWallet 2 show
  testFail runWallet 2 transfer 100 $PUB
  testOK runBA mint $bc $key $PUB2 1000
  testGrep "Balance is: 1000" runWallet 2 show
  testFail runWallet 2 transfer 10000 $PUB
  testFail runWallet 2 transfer 100 $PUB2
  testOK runWallet 2 transfer 100 $PUB
  testGrep "Balance is: 900" runWallet 2 show
  testGrep "Balance is: 1100" runWallet 1 show
}

runBA(){
  ./bcadmin -c config/ --debug $DBG_BA "$@"
}

runWallet(){
  wn=$1
  shift
  ./wallet -c wallet$wn/ --debug $DBG_BA "$@"
}

main
