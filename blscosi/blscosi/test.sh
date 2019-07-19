#!/usr/bin/env bash

DBG_TEST=1
# Debug-level for app
DBG_APP=2
CLEANBUILD=1

. ../../libtest.sh

main(){
    startTest
    for n in $(seq $NBR); do
        srv=srv$n
        rm -rf $srv
        mkdir $srv
        cl=cl$n
        rm -rf $cl
        mkdir $cl
    done
    run testBuild
    run testServerCfg
    run testSignFile
    run testCheck
    run testReconnect
    stopTest
}

testReconnect(){
    for s in 1 2; do
        setupServers 1
        testOut "Running first sign"
        echo "My Test Message File" > foo.txt
        testOK runCl 1 sign foo.txt
        testOut "Killing server $s"
        pkill -9 -f "c srv$s/private"
        testFail runCl 1 sign foo.txt
        testOut "Starting server $s again"
        runSrv $s
        testOK runCl 1 sign foo.txt
        pkill -9 -f ./blscosi
    done
}

testCheck(){
    setupServers 1
    testOK runCl 1 check
    runSrvCfg 3
    cat srv3/public.toml >> cl1/servers.toml
    testFail runCl 1 check
}

testSignFile(){
    setupServers 1
    echo "Running first sign"
    echo "My Test Message File" > foo.txt
    echo "My Second Test Message File" > bar.txt
    runCl 1 sign foo.txt
    echo "Running second sign"
    runCl 1 sign foo.txt -o cl1/signature
    testOK runCl 1 verify foo.txt -s cl1/signature
    testFail runCl 1 verify bar.txt -s cl1/signature
    rm foo.txt
    rm bar.txt
}

testServerCfg(){
    runSrvCfg 1
    pkill -9 blscosi
    testFile srv1/private.toml
}

testBuild(){
    testOK ./blscosi help
}

setupServers(){
    CLIENT=$1
    OOUT=$OUT
    OUT=/tmp/config
    SERVERS=cl$CLIENT/servers.toml
    rm -f srv1/*
    rm -f srv2/*
    runSrvCfg 1
    cp srv1/public.toml $SERVERS
    runSrvCfg 2
    echo >> $SERVERS
    cat srv2/public.toml >> $SERVERS
    runSrv 1
    runSrv 2
    OUT=$OOUT
}

runCl(){
    local D=cl$1/servers.toml
    shift
    echo "Running Client with $D $@"
    dbgRun ./blscosi -d $DBG_APP $@ -g $D
}

runSrvCfg(){
    echo -e "localhost:200$(( 2 * $1 ))\nCosi $1\n$(pwd)/srv$1\n" | ./blscosi server setup > $OUT
}

runSrv(){
    ./blscosi -d $DBG_SRV server -c srv$1/private.toml &

    sleep 10
}

main
