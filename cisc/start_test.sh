#!/usr/bin/env bash

DBG_TEST=2
# Debug-level for app
DBG_APP=2
 #DBG_SRV=2
# Needs 4 clients
#NBR=4

. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

main(){
  startTest
  buildConode github.com/dedis/cothority/identity
  runCoBG 1 2 3

  testOK runCl 1 link pin localhost:2002
  local pin=$( grep PIN ${COLOG}1.log | sed -e "s/.* //" )
  runCl 1 link pin localhost:2002 $pin
  runDbgCl 0 1 skipchain create -name client1 public.toml
  runGrepSed ID "s/.* //" runDbgCl 2 1 data list
  runCl 1 kv add 1 2
  runCl 1 kv add 1 2
  runCl 1 kv add 1 2
  echo $SED > genesis.txt
  sleep 3600
}


runCl(){
  local D=cl$1
  shift
  dbgRun ./cisc -d $DBG_APP -c $D --cs $D $@
}

runDbgCl(){
  local DBG=$1
  local CFG=cl$2
  shift 2
  if [ "$DBG" = 0 ]; then
    ./cisc -d $DBG -c $CFG --cs $CFG $@ > /dev/null || fail "error in command: $@"
  else
    ./cisc -d $DBG -c $CFG --cs $CFG $@ || fail "error in command: $@"
  fi
}

main
