#!/usr/bin/env bash

DBG_SHOW=1
#STATICDIR=test
. lib/test/libtest.sh
. lib/test/cothorityd.sh

main(){
    startTest
    build
    test Build
    test Network
    stopTest
}

testNetwork(){
    cothoritySetup
    cp group.toml cl1
    testOut "Running network"
    testGrep "Available_Services" runCl 1
    testGrep "Available_Services" runCl 1
}

testBuild(){
    testOK ./status --help
    testOK ./cothorityd --help
}

runCl(){
    D=cl$1/group.toml
    shift
    dbgRun ./status -d 0 -g $D $@
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
    testOut "Building in $DIR"
    for app in status cothorityd; do
        if [ ! -e $app -o "$BUILD" ]; then
            go build -o $app $BUILDDIR/$app/*go
        fi
    done
    for n in $(seq $NBR); do
        srv=srv$n
        rm -rf $srv
        mkdir $srv
        cl=cl$n
        rm -rf $cl
        mkdir $cl
    done
}

if [ "$1" -a "$STATICDIR" ]; then
    rm -f $STATICDIR/{cothorityd,status}
fi

main
