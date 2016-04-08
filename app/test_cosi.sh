#!/usr/bin/env bash

# highest number of servers and clients
NBR=3
# Use for suppressing building if that directory exists
#STATICDIR=test
# If set, always build
BUILD=z
# Debug-level for server
DBG_SRV=1
# Debug-level for client
DBG_CLIENT=1
# Debug running
DBG_RUN=y
# where the output should go
if [ "$DBG_RUN" ]; then
    OUT=/dev/stdout
else
    OUT=/dev/null
fi

main(){
    build
    test Build
    test ServerCfg
    test SignMsg
    echo "Success"
    cleanup
}

testSignMsg(){
    setupServers 1
    runCl 1 sign msg testCosi | tail -n 5 > cl1/signature
    testOK runCl 1 verify msg testCosi -sig cl1/signature
    testFail runCl 1 verify msg testCosi2 -sig cl1/signature
}

testServerCfg(){
    runSrvCfg 1 &
    sleep .5
    pkill cosid
    testFile srv1/config.toml
}

testBuild(){
    testOK ./cosi help
    testOK ./cosid help
}

setupServers(){
    CLIENT=$1
    OOUT=OUT
    OUT=/tmp/config
    SERVERS=cl$CLIENT/servers.toml
    runSrvCfg 1 &
    sleep .5
    tail -n 5 $OUT > $SERVERS
    runSrvCfg 2 &
    sleep .5
    tail -n 5 $OUT >> $SERVERS
    OUT=OOUT
}

runCl(){
    D=cl$1/servers.toml
    shift
    dbgRun "Running Client with $D $@"
    ./cosi -d $DBG_CLIENT -s $D $@
}

runSrvCfg(){
    echo -e "localhost:200$1\nsrv$1/config.toml\n" | runSrv $1 > $OUT
}

runSrv(){
    ./cosid -d $DBG_SRV -c srv$1/config.toml
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
    for app in cosi cosid; do
        if [ ! -e $app -o "$BUILD" ]; then
            go build $BUILDDIR/$app/$app.go
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

test(){
    echo "Testing $1"
    test$1
}

testOK(){
    if ! $@ > /dev/null; then
        fail "starting $@ failed"
    fi
}

testFail(){
    if $@ > /dev/null; then
        fail "starting $@ failed"
    fi
}

testFile(){
    if [ ! -f $1 ]; then
        fail "file $1 is not here"
    fi
}

testGrep(){
    S=$1
    shift
    if ! $@ | grep -q "$S"; then
        fail "Didn't find '$S' in output of '$@'"
    fi
}

testNGrep(){
    S=$1
    shift
    if $@ | grep -q "$S"; then
        fail "Found '$S' in output of '$@'"
    fi
}

dbgRun(){
    if [ "$DBG_RUN" ]; then
        echo $@
    fi
}

fail(){
    echo
    echo -e "\tFAILED: $@"
    cleanup
    exit 1
}

cleanup(){
    pkill cosi 2> /dev/null
    if [ ! "$STATICDIR" ]; then
        echo "removing $DIR"
        rm -rf $DIR
    fi
    sleep .5
}

main
