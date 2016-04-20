#!/usr/bin/env bash

. ./libtest.sh

tails=8

main(){
    startTest
    build
    test Build
    test ServerCfg
    test SignMsg
    stopTest
}

testSignMsg(){
    setupServers 1
    echo $OUT
    echo "Running first sign"
    runCl 1 sign msg testCosi > $OUT
    echo "Running second sign"
    runCl 1 sign msg testCosi | tail -n 5 > cl1/signature
    runCl 1 verify msg testCosi -sig cl1/signature > $OUT
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
    testOK ./cothorityd help
}

setupServers(){
    CLIENT=$1
    OOUT=$OUT
    OUT=/tmp/config
    SERVERS=cl$CLIENT/servers.toml
    runSrvCfg 1 &
    sleep .5
    tail -n $tails $OUT > $SERVERS
    runSrvCfg 2 &
    sleep .5
    tail -n $tails $OUT >> $SERVERS
    OUT=$OOUT
}

runCl(){
    D=cl$1/servers.toml
    shift
    dbgRun "Running Client with $D $@"
    ./cosi -d $DBG_CLIENT -s $D $@
}

runSrvCfg(){
    echo -e "127.0.0.1:200$1\nsrv$1/config.toml\n" | runSrv $1 > $OUT
}

runSrv(){
    ./cothorityd -d $DBG_SRV -c srv$1/config.toml
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
    for app in cosi cothorityd; do
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

main
