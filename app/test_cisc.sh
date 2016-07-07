#!/usr/bin/env bash

DBG_SHOW=2
# Debug-level for app
DBG_APP=3
# Uncomment to build in local dir
STATICDIR=test

. lib/test/libtest.sh
. lib/test/cothorityd.sh

main(){
    startTest
    build
#    test Build
#    test IdCreate
#	test DataList
	test IdConnect
    stopTest
}

testIdConnect(){
	clientSetup
	dbgOut "ID of client 1 is $ID"
	testFail runCl 2 id co
	echo test > test.toml
	testFail runCl 2 id co test.toml
	testFail runCl 2 id co group.toml
	testOK runCl 2 id co group.toml $ID client2
	own2="Owner: client2"
	testNGrep "$own2" runCl 2 data ls
	testGrep "$own2" runCl 2 data lsp
	testNGrep "$own2" runCl 1 data ls
	testGrep "$own2" runCl 1 data lsp
}

testDataList(){
	clientSetup
	testGrep "name: client1" runCl 1 data ls
	testReGrep "ID: [0-9a-f]"
}

clientSetup(){
	cothoritySetup
	DBG_OLD=$DBG_SHOW
    DBG_SHOW=2
    testOK runCl 1 id cr group.toml client1
    runGrepSed ID "s/.* //" runCl 1 data ls
    ID=$SED
    DBG_SHOW=$DBG_OLD
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

testBuild(){
    testOK dbgRun ./cothorityd --help
    testOK dbgRun ./cisc -c cl1 -cs cl1 --help
}

runCl(){
    D=cl$1
    shift
    dbgRun ./cisc -d $DBG_APP -c $D --cs $D $@
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
        rm -f $cl/*bin
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
