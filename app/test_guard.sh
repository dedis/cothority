#!/usr/bin/env bash

DBG_SHOW=1
#STATICDIR=test
. lib/test/libtest.sh
. lib/test/cothorityd.sh

main(){
    startTest
    build
#    test Build
    test Network
    stopTest
}

testNetwork(){
    cothoritySetup
    cp group.toml cl1
    testOut "Running Guard"
    testOK runCl 1 su cl1/group.toml
    testOK runCl 1 s evan dadada Hello
    testFail runCl 1 r evan dadadas
    testOK runCl 1 r evan dadada
    testNGrep "Hello" runCl 1 r evan dadadas
    testGrep "Hello" runCl 1 r evan dadada

}

testBuild(){
    testOK ./guard --help
    testOK ./cothorityd --help
}

runCl(){
    D=cl$1/group.toml
    shift
    dbgRun ./guard -d 0 $@
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
    for app in guard cothorityd; do
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
    rm -f $STATICDIR/{cothorityd,guard}
fi

main
