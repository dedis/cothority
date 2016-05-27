#!/usr/bin/env bash

. lib/test/libtest.sh
. lib/test/cothorityd.sh
DBG_SHOW=1

main(){
    startTest
    build
    test Build
    test SignFile
    stopTest
}

testSignFile(){
    cothoritySetup
    cp group.toml cl1
    testOut "Running first sign"
    echo "My Test Message File" > foo.txt
    echo "My Second Test Message File" > bar.txt
    testOK runCl 1 sign foo.txt
    testOut "Running second sign"
    testOK runCl 1 sign foo.txt -o cl1/signature
    testOK runCl 1 verify foo.txt -s cl1/signature
    testFail runCl 1 verify bar.txt -s cl1/signature
    rm foo.txt
    rm bar.txt
}

testBuild(){
    testOK ./cosi --help
    testOK ./cothorityd --help
}

runCl(){
    D=cl$1/group.toml
    shift
    dbgRun ./cosi -d 3 -g $D $@
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
    for app in cosi cothorityd; do
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
    rm -f $STATICDIR/{cothorityd,cosi}
fi

main
