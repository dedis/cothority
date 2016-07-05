#!/usr/bin/env bash

DBG_SHOW=1
# Debug-level for server
DBG_SRV=1
# Debug-level for client
DBG_CLIENT=1
# Uncomment to build in local dir
STATICDIR=test

. lib/test/libtest.sh
. lib/test/cothorityd.sh

main(){
    startTest
    build
    test Build
    #test ClientSetup
    stopTest
}

testClientSetup(){
    cothoritySetup
    testOK runCl 1 setup group.toml
    testFile cl1/config.bin
}

testBuild(){
    testOK dbgRun ./cothorityd --help
    testOK dbgRun ./cisc -c cl1 -cs cl1 --help
}

runCl(){
    D=cl$1
    shift
    dbgRun ./cisc -d $DBG_CLIENT -c $D --cs $D $@
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
        srv=srv$n
        if [ -d $srv ]; then
            rm -f $srv/*bin
        else
            mkdir $srv
            ssh-keygen -t rsa -b 4096 -N "" -f $srv/ssh_host_rsa_key > /dev/null
        fi

        cl=cl$n
        if [ -d $cl ]; then
            rm -f $cl/*bin
        else
            mkdir $cl
            ssh-keygen -t rsa -b 4096 -N "" -f $cl/id_rsa > /dev/null
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
