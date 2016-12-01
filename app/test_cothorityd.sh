#!/usr/bin/env bash

DBG_SHOW=2
# Debug-level for server
DBG_SRV=0
# For easier debugging
#STATICDIR=test

. lib/test/libtest.sh
. lib/test/cothorityd.sh

main(){
    startTest
    build
    test Build
    test Cothorityd
    stopTest
}

testCothorityd(){
    runCoCfg 1
    runCoCfg 2
    runCoCfg 3
    runCoBG 1
    runCoBG 2
    sleep 1
    cp co1/group.toml .
    tail -n 4 co2/group.toml >> group.toml
    testOK runCo 1 check -g group.toml
    tail -n 4 co3/group.toml >> group.toml
    testFail runCo 1 check -g group.toml
}

testBuild(){
    testOK dbgRun ./cothorityd --help
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

    for appdir in $BUILDDIR/cothorityd; do
        app=$(basename $appdir)
        if [ ! -e $app -o "$BUILD" ]; then
            if ! go build -o $app $appdir/*.go; then
                fail "Couldn't build $appdir"
            fi
        fi
    done
    echo "Creating keys"
    for n in $(seq $NBR); do
        co=co$n
        rm -f $co/*
        mkdir -p $co
    done
}

if [ "$1" -a "$STATICDIR" ]; then
    rm -f $STATICDIR/cothorityd
fi

main
