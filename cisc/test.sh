#!/usr/bin/env bash

DBG_TEST=1
# Debug-level for app
DBG_APP=2
# DBG_SRV=2
# Needs 4 clients
NBR=4
PACKAGE_POP_GO="github.com/dedis/cothority/pop"
PACKAGE_POP="$(go env GOPATH)/src/$PACKAGE_POP_GO"
pop=`basename $PACKAGE_POP`
PACKAGE_IDEN="github.com/dedis/cothority/identity"

. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

main(){
    startTest
    addr=()
    addr[1]=127.0.0.1:2002
    addr[2]=127.0.0.1:2004
    addr[3]=127.0.0.1:2006
    buildKeys
    buildConode github.com/dedis/cothority/cosi/service $PACKAGE_IDEN $PACKAGE_POP_GO/service
    build $PACKAGE_POP
    createFinal 2 > /dev/null
    createToken 2

    test Build
    test Link
    test Store
    test Add
    test ClientSetup
    test IdKeyPair
    test IdCreate
    test IdCreate2
    test ConfigList
    test ConfigVote
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
    testOK runCl 1 config vote y
    testOK runCl 2 config vote y

    testOK runCl 1 id rm client3
    testOK runCl 2 config vote y

    testFail runCl 3 ssh add service1
    testOK runCl 1 config update
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
    echo ID is $ID
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
    testFail runCl 1 config vote y
    testOK runCl 2 config update
    testOK runCl 2 config vote y
    testOK runCl 1 config update
    testGrep key1 runCl 1 kv ls
    testGrep key2 runCl 1 kv ls
    testFail runCl 1 kv rm key3
    testOK runCl 1 kv rm key2
    testFail runCl 1 config vote y
    testOK runCl 2 config update
    testOK runCl 2 config vote y
    testNGrep key2 runCl 2 kv ls
    testOK runCl 1 config update
    testNGrep key2 runCl 2 kv ls
}

testKeyAddWeb(){
  clientSetup 2
  mkdir dedis
  echo "<html>DEDIS rocks</html>" > dedis/index.html
  testOK runCl 1 kv addWeb dedis/index.html
  testOK runCl 2 config vote y
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
        if [ $C != 1 ]; then
            testFail runCl 1 config vote y
        fi
        if [ $C -gt 1 ]; then
            testNGrep key1 runCl 2 kv ls
            testOK runCl 2 config update
            testOK runCl 2 config vote y
            testGrep key1 runCl 2 kv ls
        fi
        testOK runCl 1 config update
        testGrep key1 runCl 1 kv ls
        testReGrep key2 runCl 1 kv ls
        cleanup
    done
}

testKeyAdd(){
    clientSetup 2
    testNGrep key1 runCl 1 kv ls
    testOK runCl 1 kv add key1 value1
    testFail runCl 1 config vote y
    testGrep key1 runCl 1 config ls -p
    testOK runCl 2 config update
    testNGrep key1 runCl 2 kv ls
    testGrep key1 runCl 2 config ls -p
    testOK runCl 2 config update
    testOK runCl 2 config vote y
    testGrep key1 runCl 2 kv ls
    testOK runCl 1 config update
    testGrep key1 runCl 1 kv ls
}

testIdDel(){
    clientSetup 3
    testOK runCl 2 ssh add server2
    testOK runCl 1 config vote y
    testGrep client2 runCl 1 config ls
    testGrep server2 runCl 1 config ls
    testOK runCl 1 id del client2
    testOK runCl 3 config vote y
    testNGrep client2 runCl 3 config ls
    testOK runCl 1 config update
    testNGrep client2 runCl 1 config ls
    testReNGrep server2
    testFail runCl 2 ssh add server
    testOK runCl 2 config update
}

testIdConnect(){
    clientSetup
    dbgOut "Connecting client_2 to ID of client_1: $ID"
    testFail runCl 2 id co
    echo test > test.toml
    testFail runCl 2 id co test.toml
    testFail runCl 2 id co public.toml
    testOK runCl 2 id co public.toml $ID client2
    runGrepSed "Public key" "s/.* //" runCl 2 id co public.toml $ID client2
    PUBLIC=$SED
    if [ -z "$PUBLIC" ]; then
        fail "no public keys received"
    fi
    own2="Connected device client2"
    testNGrep "$own2" runCl 2 config ls
    testOK runCl 2 config update
    testGrep "Owner: client2" runCl 2 config ls -p

    dbgOut "Voting with client_1 - first reject then accept"
    echo "n" | testGrep $PUBLIC runCl 1 config vote
    dbgOut
    echo "n" | testNGrep a$PUBLIC runCl 1 config vote
    dbgOut
    testOK runCl 1 config vote n
    testNGrep "$own2" runCl 1 config ls
    testOK runCl 2 config update
    testNGrep "$own2" runCl 2 config ls

    testOK runCl 1 config vote y
    testGrep "$own2" runCl 1 config ls
    testGrep "$own2" runCl 2 config ls
}

testConfigVote(){
    clientSetup 2
    testOK runCl 1 kv add one two
    testFail runCl 1 config vote y
    testNGrep one runCl 1 kv ls

    testOK runCl 2 config vote n
    testNGrep one runCl 2 kv ls
    echo y | testOK runCl 2 config vote

    testOK runCl 1 kv add three four
    testNGrep three runCl 1 kv ls
    echo "n" | testOK runCl 2 config vote
    testNGrep three runCl 1 kv ls
    echo "y" | testOK runCl 2 config vote
    testGrep three runCl 1 kv ls
    testGrep three runCl 2 kv ls

    testOK runCl 1 kv add five six
    echo y | testOK runCl 2 config vote
    testGrep five runCl 1 kv ls
    testGrep five runCl 2 kv ls
}

testConfigList(){
    clientSetup
    testGrep "name: client1" runCl 1 config ls
    testReGrep "ID: [0-9a-f]"
}

testIdCreate(){
    runCoBG 1 2 3
    testFail runCl 1 id cr -t PoP public.toml token.toml
    runStore 3
    testFail runCl 1 id cr
    echo test > test.toml
    testFail runCl 1 id cr test.toml
    testFail runCl 1 id cr -t PoP test.toml token.toml
    cat token.toml > test_token.toml
    sed -i 's/^Private = .*$/Private = "abcd"/'  test_token.toml
    testFail runCl 1 id cr -t PoP test_token.toml public.toml

    testOK runCl 1 id cr -t PoP public.toml token.toml
    testFile cl1/config.bin
    testGrep $(hostname) runCl 1 id cr -t PoP public.toml token.toml
    testGrep client1 runCl 1 id cr -t PoP public.toml token.toml client1

    for i in {1..2}; do
        testOK runCl 1 id cr -t PoP public.toml token.toml
    done
    # run out of skipchain creation limit
    testFail runCl 1 id cr -t PoP public.toml token.toml
    rm cl1/config.bin
}

testIdCreate2(){
    runCoBG 1 2 3
    local KP
    KP=$( mktemp )
    runDbgCl 2 1 id kp > $KP
    local pub=$( grep Public $KP | sed -e "s/.* //")
    local priv=$( grep Private $KP | sed -e "s/.* //")
    runDbgCl 2 1 id kp > $KP
    local pub1=$( grep Public $KP | sed -e "s/.* //")
    pubs="\[\"$pub\",\"$pub1\"\]"

    testFail runCl 1 id cr -t Public public.toml $priv
    runAdd 3 $pubs
    testFail runCl 1 id cr -t Public public.toml
    testOK runCl 1 id cr -t Public public.toml $priv
    testFile cl1/config.bin
    testGrep $(hostname) runCl 1 id cr -t Public public.toml $priv
    testGrep client1 runCl 1 id cr -t Public public.toml $priv client1

    for i in {1..2}; do
        testOK runCl 1 id cr -t Public public.toml $priv
    done
    # run out of skipchain creation limit
    testFail runCl 1 id cr -t Public public.toml $priv
    rm cl1/config.bin
}

testIdKeyPair(){
    testOK runCl 1 id kp
    runDbgCl 2 1 id kp > keypair.1
    runDbgCl 2 1 id kp > keypair.2
    cmp keypair.1 keypair.2
    testOK [ $? -eq 1 ]
}

testClientSetup(){
    MAXCLIENTS=3
    for t in $( seq $MAXCLIENTS ); do
        testOut "Starting $t clients"
        clientSetup $t
        for u in $( seq $MAXCLIENTS ); do
            if [ $u -le $t ]; then
                testGrep client1 runCl $u config ls
            else
                testFail runCl $u config ls
            fi
        done
        cleanup
    done
}

runStore(){
    runLink $1
    for (( i=1; i<=$1; i++ ))
    do
        runCl 1 admin store -t PoP final.toml ${addr[$i]}
    done
}

runAdd() {
    runLink $1
    for (( i=1; i<=$1; i++ ))
    do
        runCl 1 admin add $2 ${addr[$i]}
    done
}
testStore(){
    runCoBG 1
    runLink 1
    testFail runCl 1 admin store -t PoP
    testOK runCl 1 admin store -t PoP final.toml ${addr[1]}
}

testAdd(){
    runCoBG 1
    runLink 1
    local KP
    KP=$( mktemp )
    runDbgCl 2 1 id kp > $KP
    local pub=$( grep Public $KP | sed -e "s/.* //")
    testFail runCl 1 admin add $pub
    testOK runCl 1 admin  add $pub ${addr[1]}
}

runLink(){
    local KP
    local i
    for (( i=1; i<=$1; i++ ))
    do
        runCl 1 admin link ${addr[$i]}
        pin=$( grep PIN ${COLOG}$i.log | sed -e "s/.* //" )
        runCl 1 admin link ${addr[$i]} $pin
    done
}

testLink(){
    runCoBG `seq 1`

    testOK runCl 1 admin link ${addr[1]}
    testGrep PIN cat ${COLOG}1.log
    local pin=$( grep PIN ${COLOG}1.log | sed -e "s/.* //" )
    testFail runCl 1 admin link ${addr[1]} abcdefg
    testOK runCl 1 admin link ${addr[1]} $pin
    testFile cl1/config.bin
}

testBuild(){
    testOK dbgRun runCo 1 --help
    testOK dbgRun runCl 1 --help
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
    ./cisc -d $DBG -c $CFG --cs $CFG $@
}

clientSetup(){
    runCoBG `seq 3`
    local CLIENTS=${1:-0} c b
    # admin
    runStore 3
    runDbgCl 0 1 id cr -t PoP public.toml token.toml client1
    runGrepSed ID "s/.* //" runDbgCl 2 1 config ls
    ID=$SED
    echo "ID is" $ID
    if [ "$CLIENTS" -gt 1 ]; then
        for c in $( seq 2 $CLIENTS ); do
            runCl $c id co public.toml $ID client$c
            for b in 1 2; do
                if [ $b -lt $c ]; then
                    runDbgCl 0 $b config update
                    runDbgCl 0 $b config vote y
                fi
            done
        done
        for c in $( seq $CLIENTS ); do
            runDbgCl 0 $c config update
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
        sed -n "$((4*$n-3)),$((4*$n))p" public.toml >> pop_desc.toml
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
    runCoBG 1 2
    local KP
    KP=$( mktemp )
    ./$pop -d 2 attendee create > $KP
    PRIV_USER=$( grep Private $KP | sed -e "s/.* //")
    PUB_USER=$( grep Public $KP | sed -e "s/.* //")

    ./$pop -d 2 attendee create > $KP
    local pub_user1=$( grep Public $KP | sed -e "s/.* //")
    createPopDesc $1

    ./$pop -c cl1 org link ${addr[1]}
    local pin=$( grep PIN ${COLOG}1.log | sed -e "s/.* //" )
    testOK ./$pop -c cl1 org link ${addr[1]} $pin

    ./$pop -c cl2 org link ${addr[2]}
    pin=$( grep PIN ${COLOG}2.log | sed -e "s/.* //" )
    ./$pop -c cl2 org link ${addr[2]} $pin

    ./$pop -c cl1 org config pop_desc.toml
    ./$pop -c cl2 -d 2 org config pop_desc.toml > pop_hash_file
    pop_hash=$(grep config: pop_hash_file | sed -e "s/.* //")
    ./$pop -c cl1 org public $PUB_USER $pop_hash
    ./$pop -c cl2 org public $PUB_USER $pop_hash
    ./$pop -c cl1 org public $pub_user1 $pop_hash
    ./$pop -c cl2 org public $pub_user1 $pop_hash
    ./$pop -c cl1 org final $pop_hash
    DEBUG_COLOR="" ./$pop -c cl2 -d 2 org final $pop_hash | tail -n +3> final.toml
}
main
