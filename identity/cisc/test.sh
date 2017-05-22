#!/usr/bin/env bash

DBG_TEST=2
# Debug-level for app
DBG_APP=2
DBG_SRV=2
# Needs 4 clients
NBR=4

. $GOPATH/src/gopkg.in/dedis/onet.v1/app/libtest.sh

main(){
    startTest
	buildKeys
	buildConode "github.com/dedis/cothority/identity/service"
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
}

testConfigList(){
	clientSetup
	testGrep "name: client1" runCl 1 config ls
	testReGrep "ID: [0-9a-f]"
}

testIdCreate(){
	runCoBG 1 2 3
    testFail runCl 1 id cr
    echo test > test.toml
    testFail runCl 1 id cr test.toml
    testOK runCl 1 id cr public.toml
	testFile cl1/config.bin
    testGrep $(hostname) runCl 1 id cr public.toml
    testGrep client1 runCl 1 id cr public.toml client1
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
    local CLIENTS=${1:-0} c b
	runCoBG 1 2 3
	D=2
    runDbgCl $D 1 id cr public.toml client1
    runGrepSed ID "s/.* //" runDbgCl 2 1 config ls
    ID=$SED
    dbgOut "ID is $ID"
    if [ "$CLIENTS" -gt 1 ]; then
    	for c in $( seq 2 $CLIENTS ); do
    		dbgOut "Connecting client $c"
    		runCl $c id co public.toml $ID client$c
    		for b in 1 2; do
    			if [ $b -lt $c ]; then
    				dbgOut "Updating and voting for client $c"
					runDbgCl $D $b config update
					runDbgCl $D $b config vote y
				fi
			done
		done
		for c in $( seq $CLIENTS ); do
			runDbgCl 0 $c config update
		done
	fi
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

main
