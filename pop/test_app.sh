#!/usr/bin/env bash

DBG_SHOW=1
# Debug-level for app
DBG_APP=2
DBG_SRV=0
# Uncomment to build in local dir
#STATICDIR=test
# Needs 4 clients
NBR=4

TESTPATH=$GOPATH/src/github.com/dedis/onet/app/lib/test
. $TESTPATH/libtest.sh
. $TESTPATH/cothorityd.sh

main(){
    startTest
    build
	test Build
	test Main
    stopTest
}

testMain(){
	testGrep Main ./app main
}

testBuild(){
    testOK dbgRun ./app --help
}

runTmpl(){
    local D=tmpl$1
    shift
    dbgRun ./app -d $DBG_APP $D $@
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
    if [ ! -e app -o "$BUILD" ]; then
        if ! go build -o app $BUILDDIR/*.go; then
            fail "Couldn't build app"
        fi
    fi
}

if [ "$1" -a "$STATICDIR" ]; then
    rm -f $STATICDIR/{app}
fi

main
