#!/usr/bin/env bash

DBG_SHOW=1
# Debug-level for app
DBG_APP=0
DBG_SRV=0
# Uncomment to build in local dir
#STATICDIR=test
# Needs 4 clients
NBR=4

. lib/test/libtest.sh
. lib/test/cothorityd.sh

main(){
    startTest
    build
	test Build
	test ClientSetup
	test IdCreate
	test ConfigList
	test ConfigVote
	test IdConnect
	test IdDel
	test KeyAdd
	test KeyAdd2
	test KeyDel
	test SSHAdd
	test SSHDel
	test Follow
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

testFollow(){
	clientSetup 1
	echo ID is $ID
	testNFile cl3/authorized_keys
	testFail runCl 3 follow add group.toml 1234 service1
	testOK runCl 3 follow add group.toml $ID service1
	testFail grep -q service1 cl3/authorized_keys
	testNGrep client1 runCl 3 follow list
	testGrep $ID runCl 3 follow list
	testOK runCl 1 ssh add service1
	testOK runCl 3 follow update
	testOK grep -q service1 cl3/authorized_keys
	testGrep service1 runCl 3 follow list
	testReGrep client1
	testOK runCl 3 follow rm $ID
	testNGrep client1 runCl 3 follow list
	testReNGrep service1
	testFail grep -q service1 cl3/authorized_keys
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
	testOK runCl 1 config vote y
	testOK runCl 2 config update
	testOK runCl 2 config vote y
	testOK runCl 1 config update
	testGrep key1 runCl 1 kv ls
	testGrep key2 runCl 1 kv ls
	testFail runCl 1 kv rm key3
	testOK runCl 1 kv rm key2
	testOK runCl 1 config vote y
	testOK runCl 2 config update
	testOK runCl 2 config vote y
	testNGrep key2 runCl 2 kv ls
	testOK runCl 1 config update
	testNGrep key2 runCl 2 kv ls
}

testKeyAdd2(){
	MAXCLIENTS=3
	for C in $( seq $MAXCLIENTS ); do
		testOut "Running with $C devices"
		clientSetup $C
		testOK runCl 1 kv add key1 value1
		testOK runCl 1 kv add key2 value2
		testOK runCl 1 config vote y
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
	testOK runCl 1 config vote y
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
	testFail runCl 2 id co group.toml
	testOK runCl 2 id co group.toml $ID client2
	runGrepSed "Public key" "s/.* //" runCl 2 id co group.toml $ID client2
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
	clientSetup
	testOK runCl 1 kv add one two
	testNGrep one runCl 1 kv ls
	testOK runCl 1 config vote n
	testNGrep one runCl 1 kv ls

	testOK runCl 1 config vote y
	testGrep one runCl 1 kv ls

	testOK runCl 1 kv add three four
	testNGrep three runCl 1 kv ls
	echo "n" | testOK runCl 1 config vote
	testNGrep three runCl 1 kv ls
	echo "y" | testOK runCl 1 config vote
	testGrep three runCl 1 kv ls
}

testConfigList(){
	clientSetup
	testGrep "name: client1" runCl 1 config ls
	testReGrep "ID: [0-9a-f]"
}

testIdCreate(){
    cothoritySetup
    testFail runCl 1 id cr
    echo test > test.toml
    testFail runCl 1 id cr test.toml
    testOK runCl 1 id cr group.toml
	testFile cl1/config.bin
    testGrep $(hostname) runCl 1 id cr group.toml
    testGrep client1 runCl 1 id cr group.toml client1
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

testBuild(){
    testOK dbgRun ./cothorityd --help
    testOK dbgRun ./cisc -c cl1 -cs cl1 --help
}

runCl(){
    local D=cl$1
    shift
    dbgRun ./cisc -d $DBG_APP -c $D --cs $D $@
}

clientSetup(){
    local CLIENTS=${1:-0} c b
	cothoritySetup
	local DBG_OLD=$DBG_SHOW
    DBG_SHOW=2
    testOK runCl 1 id cr group.toml client1
    runGrepSed ID "s/.* //" runCl 1 config ls
    ID=$SED
    if [ "$CLIENTS" -gt 1 ]; then
    	for c in $( seq 2 $CLIENTS ); do
    		testOK runCl $c id co group.toml $ID client$c
    		for b in 1 2; do
    			if [ $b -lt $c ]; then
					testOK runCl $b config update
					testOK runCl $b config vote y
				fi
			done
		done
		for c in $( seq $CLIENTS ); do
			testOK runCl $c config update
		done
	fi
    DBG_SHOW=$DBG_OLD
}

build(){
    BUILDDIR=$(pwd)
    if [ "$STATICDIR" ]; then
        DIR=$STATICDIR
    else
        DIR=$(mktemp -d)
    fi
    mkdir -p $DIR
    cd $DIR
    echo "Building in $DIR"
    for app in cothorityd cisc; do
        if [ ! -e $app -o "$BUILD" ]; then
            if ! go build -o $app $BUILDDIR/$app/*.go; then
                fail "Couldn't build $app"
            fi
        fi
    done
    echo "Creating keys"
    for n in $(seq $NBR); do
        co=co$n
        rm -f $co/*bin
        mkdir -p $co

        cl=cl$n
        rm -f $cl/*bin $cl/config $cl/*.{pub,key} $cl/auth*
        mkdir -p $cl
        key=$cl/id_rsa
        if [ ! -f $key ]; then
        	ssh-keygen -t rsa -b 4096 -N "" -f $key > /dev/null
        fi

        co=co$n
        rm -f $co/*
        mkdir -p $co
    done
}

if [ "$1" -a "$STATICDIR" ]; then
    rm -f $STATICDIR/{cothorityd,cisc}
fi

main
