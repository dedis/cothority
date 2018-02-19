#!/usr/bin/env bash

DBG_TEST=1
# Debug-level for app
DBG_APP=2
# DBG_SRV=2
# Needs 4 clients
NBR=4
PACKAGE_POP_GO="github.com/dedis/cothority/pop"
PACKAGE_POP="$(go env GOPATH)/src/$PACKAGE_POP_GO"
PACKAGE_SCMGR_GO="github.com/dedis/cothority/scmgr"
PACKAGE_SCMGR="$(go env GOPATH)/src/$PACKAGE_SCMGR_GO"
pop=./`basename $PACKAGE_POP`
scmgr=./`basename $PACKAGE_SCMGR`
PACKAGE_IDEN="github.com/dedis/cothority/identity"

. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

main(){
  startTest
  addr=()
  addr[1]=localhost:2002
  addr[2]=localhost:2004
  addr[3]=localhost:2006
  buildKeys
  buildConode github.com/dedis/cothority/cosi/service $PACKAGE_IDEN $PACKAGE_POP_GO/service
  build $PACKAGE_POP
  build $PACKAGE_SCMGR
  createFinal 2 > /dev/null
  createToken 2

  test Build
  test Link
  test Final
  test ClientSetup
  test ScCreate
  test ScCreate2
  test ScCreate3
  test DataList
  test DataVote
  test DataRoster
  test IdConnect
  test IdDel
  test KeyAdd
  test KeyAdd2
  test KeyAddWeb
  test KeyDel
  test SSHAdd
  test SSHDel
  test Follow
  test SymLink
  test Revoke
  stopTest
}

testRevoke(){
  clientSetup 3
  testOK runCl 3 ssh add service1
  testOK runCl 1 data vote -yes
  testOK runCl 2 data vote -yes

  testOK runCl 1 skipchain del client3
  testOK runCl 2 data vote -yes

  testFail runCl 3 ssh add service1
  testOK runCl 1 data update
}

testSymLink(){
  clientSetup 1
  testFail [ -L cl3/authorized_keys ]
  testNFile cl3/authorized_keys.cisc
  testOK runCl 3 follow add public.toml $ID service1
  testOK [ -L cl3/authorized_keys ]
  testFile cl3/authorized_keys.cisc
  rm cl3/authorized*
  echo my_secret_ssh_key > cl3/authorized_keys
  runCl 3 follow add public.toml $ID service1
  testFile cl3/authorized_keys
  testFile cl3/authorized_keys.cisc
}

testFollow(){
  clientSetup 1
  testNFile cl3/authorized_keys.cisc
  testFail runCl 3 follow add public.toml 1234 service1
  testOK runCl 3 follow add public.toml $ID service1
  testFail grep -q service1 cl3/authorized_keys.cisc
  testNGrep client1 runCl 3 follow list
  testGrep $ID runCl 3 follow list
  testOK runCl 1 ssh add service1
  testOK runCl 3 follow update
  testOK grep -q service1 cl3/authorized_keys.cisc
  testGrep service1 runCl 3 follow list
  testReGrep client1
  testOK runCl 3 follow rm $ID
  testNGrep client1 runCl 3 follow list
  testReNGrep service1
  testFail grep -q service1 cl3/authorized_keys.cisc
}

testSSHDel(){
  clientSetup 1
  testOK runCl 1 ssh add service1
  testOK runCl 1 ssh add -a s2 service2
  testOK runCl 1 ssh add -a s3 service3
  testGrep service1 runCl 1 ssh ls
  testReGrep service2
  testReGrep service3
  testOK runCl 1 ssh rm service1
  testNGrep service1 runCl 1 ssh ls
  testReGrep service2
  testReGrep service3
  testOK runCl 1 ssh rm s2
  testNGrep service2 runCl 1 ssh ls
  testOK runCl 1 ssh rm service3
  testNGrep service3 runCl 1 ssh ls
}

testSSHAdd(){
  clientSetup 1
  testOK runCl 1 ssh add service1
  testFileGrep "Host service1\n\tHostName service1\n\tIdentityFile cl1/key_service1" cl1/config
  testFile cl1/key_service1.pub
  testFile cl1/key_service1
  testGrep service1 runCl 1 ssh ls
  testReGrep client1
  testOK runCl 1 ssh add -a s2 service2
  testFileGrep "Host s2\n\tHostName service2\n\tIdentityFile cl1/key_s2" cl1/config
  testFile cl1/key_s2.pub
  testFile cl1/key_s2
  testGrep service2 runCl 1 ssh ls
  testReGrep client1

  testOK runCl 1 ssh add -sec 4096 service3
  if [ $( wc -c < "cl1/key_service1.pub" ) -ne 381 ]; then
    fail "Public-key of standard (2048) bit key should be of length 381"
  fi
  if [ $( wc -c < "cl1/key_service3.pub" ) -ne 725 ]; then
    fail "Public-key of standard (4096) bit key should be of length 725"
  fi
}

testKeyDel(){
  clientSetup 2
  testOK runCl 1 kv add key1 value1
  testOK runCl 1 kv add key2 value2
  testOK runCl 2 data update
  testOK runCl 2 data vote -yes
  testOK runCl 1 data update
  testGrep key1 runCl 1 kv ls
  testGrep key2 runCl 1 kv ls
  testFail runCl 1 kv rm key3
  testOK runCl 1 kv rm key2
  testOK runCl 2 data update
  testOK runCl 2 data vote -yes
  testNGrep key2 runCl 2 kv ls
  testOK runCl 1 data update
  testNGrep key2 runCl 2 kv ls
}

testKeyAddWeb(){
  clientSetup 2
  mkdir dedis
  echo "<html>DEDIS rocks</html>" > dedis/index.html
  testOK runCl 1 web dedis/index.html
  testOK runCl 2 data vote -yes
  testGrep "html:dedis:index.html" runCl 1 kv list
  testGrep "DEDIS rocks" runCl 1 kv value "html:dedis:index.html"
  rm -rf dedis/index.html
}

testKeyAdd2(){
  MAXCLIENTS=3
  for C in $( seq $MAXCLIENTS ); do
    testOut "Running with $C devices"
    clientSetup $C
    testOK runCl 1 kv add key1 value1
    testOK runCl 1 kv add key2 value2
    if [ $C -gt 1 ]; then
      testNGrep key1 runCl 2 kv ls
      testOK runCl 2 data update
      testOK runCl 2 data vote -yes
      testGrep key1 runCl 2 kv ls
    fi
    testOK runCl 1 data update
    testGrep key1 runCl 1 kv ls
    testReGrep key2 runCl 1 kv ls
    cleanup
  done
}

testKeyAdd(){
  clientSetup 2
  testNGrep key1 runCl 1 kv ls
  testOK runCl 1 kv add key1 value1
  testGrep key1 runCl 1 data list -p
  testOK runCl 2 data update
  testNGrep key1 runCl 2 kv ls
  testGrep key1 runCl 2 data list -p
  testOK runCl 2 data update
  testOK runCl 2 data vote -yes
  testGrep key1 runCl 2 kv ls
  testOK runCl 1 data update
  testGrep key1 runCl 1 kv ls
}

testIdDel(){
  clientSetup 3
  testOK runCl 2 ssh add server2
  testOK runCl 1 data vote -yes
  testGrep client2 runCl 1 data list
  testGrep server2 runCl 1 data list
  testOK runCl 1 skipchain del client2
  testOK runCl 3 data vote -yes
  testNGrep client2 runCl 3 data list
  testOK runCl 1 data update
  testNGrep client2 runCl 1 data list
  testReNGrep server2
  testFail runCl 2 ssh add server
  testOK runCl 2 data update
}

testIdConnect(){
  clientSetup
  dbgOut "Connecting client_2 to ID of client_1: $ID"
  testFail runCl 2 skipchain join
  echo test > test.toml
  testFail runCl 2 skipchain join test.toml
  testFail runCl 2 skipchain join public.toml
  testOK runCl 2 skipchain join public.toml $ID client2
  runGrepSed "Public key" "s/.* //" runCl 2 skipchain join public.toml $ID client2
  PUBLIC=$SED
  if [ -z "$PUBLIC" ]; then
    fail "no public keys received"
  fi
  own2="Connected device client2"
  testNGrep "$own2" runCl 2 data list
  testOK runCl 2 data update
  testGrep "Owner: client2" runCl 2 data list -p

  dbgOut "Voting with client_1 - first reject then accept"
  echo "n" | testGrep $PUBLIC runCl 1 data vote
  echo "n" | testNGrep a$PUBLIC runCl 1 data vote
  testOK runCl 1 data vote -no
  testNGrep "$own2" runCl 1 data list
  testOK runCl 2 data update
  testNGrep "$own2" runCl 2 data list

  testOK runCl 1 data vote -yes
  testGrep "$own2" runCl 1 data list
  testGrep "$own2" runCl 2 data list
}

testDataVote(){
  clientSetup 3
  testOK runCl 1 kv add one two
  testNGrep one runCl 1 kv ls

  testOK runCl 2 data vote -no
  testNGrep one runCl 2 kv ls
  testOK runCl 2 data vote -y

  testOK runCl 1 kv add three four
  testNGrep three runCl 1 kv ls
  testOK runCl 2 data vote -n
  testNGrep three runCl 1 kv ls
  testOK runCl 3 data vote -y
  testGrep three runCl 1 kv ls
  testGrep three runCl 2 kv ls

  testOK runCl 1 kv add five six
  testOK runCl 2 data vote -y
  testGrep five runCl 1 kv ls
  testGrep five runCl 2 kv ls
}

testDataRoster(){
  runCoBG 1 2 3
  local KP
  KP=$( mktemp )
  runDbgCl 2 1 link keypair > $KP
  local pub1=$( grep Public $KP | sed -e "s/.* //")
  local priv1=$( grep Private $KP | sed -e "s/.* //")
  runAddPublic 3 $pub1
  testOK runCl 1 skipchain create co1/public.toml
  testNGrep 2004 runCl 1 data list
  testOK runCl 1 skipchain roster public.toml
  testGrep 2004 runCl 1 data list
  testOK runCl 1 kv add one two
}

testDataList(){
  clientSetup
  testGrep "name: client1" runCl 1 data list
  testReGrep "ID: [0-9a-f]"
}

testScCreate(){
  runCoBG 1 2 3
  testFail runCl 1 skipchain create public.toml
  cat token.toml > test_token.toml
  perl -pi -e 's/^Private = .*$/Private = "abcd"/' test_token.toml
  testFail runCl 1 link addfinal test_token.toml ${addr[1]}

  runAddToken 1
  testOK runCl 1 skipchain create public.toml
  testFile cl1/config.bin
  testGrep $(hostname) runCl 1 skipchain create public.toml
  testGrep client1 runCl 1 skipchain create -name client1 public.toml

  testOK runCl 1 skipchain create public.toml

  testOK $scmgr link add co1/private.toml
  testOK runCl 1 skipchain create public.toml

  # run out of skipchain creation limit
  testFail runCl 1 skipchain create public.toml
  rm cl1/config.bin
}

testScCreate2(){
  runCoBG 1 2 3
  local KP
  KP=$( mktemp )
  runDbgCl 2 1 link keypair > $KP
  local pub1=$( grep Public $KP | sed -e "s/.* //")
  local priv1=$( grep Private $KP | sed -e "s/.* //")
  runDbgCl 2 1 link keypair > $KP
  local pub2=$( grep Public $KP | sed -e "s/.* //")
  local priv2=$( grep Private $KP | sed -e "s/.* //")

  testFail runCl 2 skipchain create -private public.toml
  testFail runCl 2 skipchain create -private $priv1 public.toml
  runAddPublic 3 $pub1
  testOK runCl 2 skipchain create -private $priv1 public.toml
  testFail runCl 2 skipchain create -private $priv2 public.toml
  testFile cl1/config.bin
  testGrep $(hostname) runCl 2 skipchain create -private $priv1 public.toml
  testGrep client1 runCl 2 skipchain create -private $priv1 -name client1 public.toml

  for i in {1..2}; do
    testOK runCl 2 skipchain create -private $priv1 public.toml
  done
  # run out of skipchain creation limit
  testFail runCl 2 skipchain create -private $priv1 public.toml
  rm cl1/config.bin
}

testScCreate3(){
  runCoBG 1 2 3

  testFail runCl 2 skipchain create -token public.toml
  testFail runCl 2 skipchain create -token token.toml public.toml
  runLink 1
  runCl 1 link addfinal final.toml ${addr[1]}
  testOK runCl 2 skipchain create -token token.toml public.toml
  testFile cl1/config.bin
  testGrep $(hostname) runCl 2 skipchain create -token token.toml public.toml
  testGrep client1 runCl 2 skipchain create -token token.toml -name client1 public.toml

  for i in {1..2}; do
    testOK runCl 2 skipchain create -token token.toml public.toml
  done
  # run out of skipchain creation limit
  testFail runCl 2 skipchain create -token token.toml public.toml
  rm cl1/config.bin
}

testClientSetup(){
  MAXCLIENTS=3
  for t in $( seq $MAXCLIENTS ); do
    dbgOut "Starting $t clients"
    clientSetup $t
    for u in $( seq $MAXCLIENTS ); do
      if [ $u -le $t ]; then
        testGrep client1 runCl $u data list
      else
        testFail runCl $u data list
      fi
    done
    cleanup
  done
}

runAddToken(){
  runLink $1
  for i in $( seq $1 ); do
    runCl 1 link addfinal final.toml ${addr[$i]}
  done
}

runAddPublic() {
  local cl=$1
  local pub=$2
  runLink $cl
  for i in $( seq $cl ); do
    runCl 1 link addpublic $pub ${addr[$i]} || fail "adding public to $i"
  done
}
testFinal(){
  runCoBG 1
  runLink 1
  testFail runCl 1 link addfinal
  testOK runCl 1 link addfinal final.toml ${addr[1]}
}

runLink(){
  local i
  for i in $( seq $1 ); do
    runCl 1 link pin ${addr[$i]} || fail "getting pin for $i"
    pin=$( grep PIN ${COLOG}$i.log | tail -n 1 | sed -e "s/.* //" )
    runCl 1 link pin ${addr[$i]} $pin || fail "linking with $i"
  done
}

testLink(){
  runCoBG 1

  testOK runCl 1 link pin ${addr[1]}
  testGrep PIN cat ${COLOG}1.log
  local pin=$( grep PIN ${COLOG}1.log | sed -e "s/.* //" )
  testFail runCl 1 link pin ${addr[1]} abcdefg
  testOK runCl 1 link pin ${addr[1]} $pin
  testFile cl1/config.bin
}

testBuild(){
  testOK dbgRun runCo 1 --help
  testOK dbgRun runCl 1 --help
}

clientSetup(){
  runCoBG `seq 3`
  local CLIENTS=${1:-0} c b
  # admin
  runAddToken 3
  runDbgCl 0 1 skipchain create -name client1 public.toml
  runGrepSed ID "s/.* //" runDbgCl 2 1 data list
  ID=$SED
  dbgOut "ID is" $ID
  if [ "$CLIENTS" -gt 1 ]; then
    for c in $( seq 2 $CLIENTS ); do
      runCl $c skipchain join public.toml $ID client$c
      for b in 1 2; do
        if [ $b -lt $c ]; then
          runDbgCl 0 $b data update
          runDbgCl 0 $b data vote -yes
        fi
      done
    done
    for c in $( seq $CLIENTS ); do
      runDbgCl 0 $c data update
    done
  fi
  rm -f */authorized_keys*
}

buildKeys(){
  testOut "Creating keys"
  for n in $(seq $NBR); do
    cl=cl$n
    rm -f $cl/*bin $cl/config $cl/*.{pub,key} $cl/auth*
    mkdir -p $cl
    key=$cl/id_rsa
    if [ ! -f $key ]; then
      ssh-keygen -t rsa -b 4096 -N "" -f $key > /dev/null
    fi
  done
}

createPopDesc(){
  cat << EOF > pop_desc.toml
Name = "Proof-of-Personhood Party"
DateTime = "2017-08-08 15:00 UTC"
Location = "Earth, City"
EOF
  local n
  for (( n=1; n<=$1; n++ ))
  do
    sed -n "$((5*$n-4)),$((5*$n))p" public.toml >> pop_desc.toml
  done
}

createToken(){
  cat << EOF > token.toml
Private = "$PRIV_USER"
Public = "$PUB_USER"
EOF
  cat << EOF >> token.toml
[Final]
EOF
  sed -n "1,3p" final.toml >> token.toml
  cat << EOF >> token.toml
[Final.Desc]
EOF
  sed -n "6,10p" final.toml >> token.toml
}

createFinal(){
  cleanup
  runCoBG 1 2
  local KP
  KP=$( mktemp )
  $pop -d 2 attendee create > $KP
  PRIV_USER=$( grep Private $KP | sed -e "s/.* //")
  PUB_USER=$( grep Public $KP | sed -e "s/.* //")

  $pop -d 2 attendee create > $KP
  local pub_user1=$( grep Public $KP | sed -e "s/.* //")
  createPopDesc $1

  $pop -c cl1 org link ${addr[1]}
  local pin=$( grep PIN ${COLOG}1.log | sed -e "s/.* //" )
  testOK $pop -c cl1 org link ${addr[1]} $pin

  $pop -c cl2 org link ${addr[2]}
  pin=$( grep PIN ${COLOG}2.log | sed -e "s/.* //" )
  $pop -c cl2 org link ${addr[2]} $pin

  $pop -c cl1 org config pop_desc.toml
  $pop -c cl2 -d 2 org config pop_desc.toml > pop_hash_file
  pop_hash=$(grep config: pop_hash_file | sed -e "s/.* //")
  $pop -c cl1 org public $PUB_USER $pop_hash
  $pop -c cl2 org public $PUB_USER $pop_hash
  $pop -c cl1 org public $pub_user1 $pop_hash
  $pop -c cl2 org public $pub_user1 $pop_hash
  $pop -c cl1 org final $pop_hash
  DEBUG_COLOR="" $pop -c cl2 -d 2 org final $pop_hash | tail -n +3> final.toml
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
