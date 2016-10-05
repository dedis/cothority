#!/usr/bin/env bash

DBG_SHOW=1
# Debug-level for app
DBG_APP=2
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
	test Main
    stopTest
}

testMain(){
	testGrep Main ./template main
}

testBuild(){
    testOK dbgRun ./template --help
}

runTmpl(){
    local D=tmpl$1
    shift
    dbgRun ./template -d $DBG_APP $D $@
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
    for app in template; do
        if [ ! -e $app -o "$BUILD" ]; then
            if ! go build -o $app $BUILDDIR/$app/*.go; then
                fail "Couldn't build $app"
            fi
        fi
    done
}

if [ "$1" -a "$STATICDIR" ]; then
    rm -f $STATICDIR/{template}
fi

main
