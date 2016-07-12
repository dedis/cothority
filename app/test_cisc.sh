#!/usr/bin/env bash

DBG_SHOW=2
# Debug-level for app
DBG_APP=2
# Uncomment to build in local dir
STATICDIR=test

. lib/test/libtest.sh
. lib/test/cothorityd.sh

main(){
    startTest
    build
#	test Build
#	test ClientSetup
#	test IdCreate
#	test DataList
#	test IdConnect
#	test KeyAdd
#	test KeyAdd2
#	test KeyDel
#	test SSHAdd
	test SSHDel
    stopTest
}

testSSHDel(){
	clientSetup 1
	testOK runCl 1 ssh add service1
	testOK runCl 1 ssh add -a s2 service2
	testOK runCl 1 ssh add -a s3 service3
	testGrep service1 runCl 1 ssh ls
	testReGrep s2
	testReGrep s3
	testOK runCl 1 ssh rm service1
	testNGrep service1 runCl 1 ssh ls
	testReGrep s2
	testReGrep s3
	testOK runCl 1 ssh rm s2
	testNGrep s2 runCl 1 ssh ls
	testFail runCl 1 ssh rm service3
	testGrep s3 runCl 1 ssh ls
}

testSSHAdd(){
	clientSetup 1
	testOK runCl 1 ssh add service1
	testFileGrep "Host service1\n\tHostName service1\n\tIdentityFile key_service1" cl1/config
	testFile cl1/key_service1.pub
	testFile cl1/key_service1
	testGrep service1 runCl 1 ssh ls
	testReGrep client1
	testOK runCl 1 ssh add -a s2 service2
	testFileGrep "Host s2\n\tHostName service2\n\tIdentityFile key_s2" cl1/config
	testFile cl1/key_s2.pub
	testFile cl1/key_s2
	testGrep s2 runCl 1 ssh ls
	testReGrep client1
}

testKeyDel(){
	clientSetup 2
	testOK runCl 1 kv add key1 value1
	testOK runCl 1 kv add key2 value2
	testOK runCl 1 data vote
	testOK runCl 2 data update
	testOK runCl 2 data vote
	testOK runCl 1 data update
	testGrep key1 runCl 1 kv ls
	testGrep key2 runCl 1 kv ls
	testFail runCl 1 kv rm key3
	testOK runCl 1 kv rm key2
	testOK runCl 1 data vote
	testOK runCl 2 data update
	testOK runCl 2 data vote
	testNGrep key2 runCl 2 kv ls
	testOK runCl 1 data update
	testNGrep key2 runCl 2 kv ls
}

testKeyAdd2(){
	MAXCLIENTS=3
	for C in $( seq $MAXCLIENTS ); do
		testOut "Running with $C devices"
		clientSetup $C
		testOK runCl 1 kv add key1 value1
		testOK runCl 1 kv add key2 value2
		testOK runCl 1 data vote
		if [ $C -gt 1 ]; then
			testNGrep key1 runCl 2 kv ls
			testOK runCl 2 data update
			testOK runCl 2 data vote
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
	testOK runCl 1 data vote
	testGrep key1 runCl 1 data ls -p
	testOK runCl 2 data update
	testNGrep key1 runCl 2 kv ls
	testGrep key1 runCl 2 data ls -p
	testOK runCl 2 data update
	testOK runCl 2 data vote
	testGrep key1 runCl 2 kv ls
	testOK runCl 1 data update
	testGrep key1 runCl 1 kv ls
}

testIdConnect(){
	clientSetup
	dbgOut "Connecting client_2 to ID of client_1: $ID"
	testFail runCl 2 id co
	echo test > test.toml
	testFail runCl 2 id co test.toml
	testFail runCl 2 id co group.toml
	testOK runCl 2 id co group.toml $ID client2
	own2="Owner: client2"
	testNGrep "$own2" runCl 2 data ls
	testOK runCl 2 data update
	testGrep "$own2" runCl 2 data ls -p

	dbgOut "Verifying client_1 is not auto-updated"
	testNGrep "$own2" runCl 1 data ls
	testNGrep "$own2" runCl 1 data ls -p
	testOK runCl 1 data update
	testGrep "$own2" runCl 1 data ls -p

	dbgOut "Voting with client_1 - first reject then accept"
	testOK runCl 1 data vote -r
	testNGrep "$own2" runCl 1 data ls
	testOK runCl 2 data update
	testNGrep "$own2" runCl 2 data ls

	testOK runCl 1 data vote
	testGrep "$own2" runCl 1 data ls
	testNGrep "$own2" runCl 2 data ls
	testOK runCl 2 data update
	testGrep "$own2" runCl 2 data ls
}

testDataList(){
	clientSetup
	testGrep "name: client1" runCl 1 data ls
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
				testGrep client1 runCl $u data ls
			else
				testFail runCl $u data ls
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
    DBG_SHOW=0
    testOK runCl 1 id cr group.toml client1
    runGrepSed ID "s/.* //" runCl 1 data ls
    ID=$SED
    if [ "$CLIENTS" -gt 1 ]; then
    	for c in $( seq 2 $CLIENTS ); do
    		testOK runCl $c id co group.toml $ID client$c
    		for b in 1 2; do
    			if [ $b -lt $c ]; then
					testOK runCl $b data update
					testOK runCl $b data vote
				fi
			done
		done
		for c in $( seq $CLIENTS ); do
			testOK runCl $c data update
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
        rm -f $cl/*bin $cl/config $cl/*.{pub,key}
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
