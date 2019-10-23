#!/usr/bin/env bash

DBG_TEST=1
# Debug-level for app
DBG_APP=2

. ../libtest.sh

main(){
    startTest
    buildConode go.dedis.ch/cothority/v4/cosi/service
    rm -rf cl*
    mkdir cl{1,2,3}
    run testBuild
    run testServerCfg
    run testSignFile
    run testCheck
    run testReconnect
    stopTest
}

testReconnect(){
    for s in 1 2; do
        runCoBG 1 2
        testOut "Running first sign"
        echo "My Test Message File" > foo.txt
        testOK runCl 1 sign foo.txt
        testOut "Killing server $s"
        pkill -9 -f "c co$s/private"
        testFail runCl 1 sign foo.txt
        testOut "Starting server $s again"
        runCoBG $s
        sleep 1
        testOK runCl 1 sign foo.txt
        cleanup
    done
}

testCheck(){
    runCoBG 1 2
    testOK runCl 1 check
    cp public.toml public.toml.backup
    cat public.toml co3/public.toml > public.toml
    testFail runCl 1 check
    mv public.toml.backup public.toml
}

testSignFile(){
    runCoBG 1 2
    echo "Running first sign"
    echo "My Test Message File" > foo.txt
    echo "My Second Test Message File" > bar.txt
    runCl 1 sign foo.txt > /dev/null
    echo "Running second sign"
    runCl 1 sign foo.txt -o cl1/signature > /dev/null
    testOK runCl 1 verify foo.txt -s cl1/signature
    testFail runCl 1 verify bar.txt -s cl1/signature
    rm foo.txt
    rm bar.txt
}

testServerCfg(){
    runCoBG 1
    pkill -9 cosi
    testFile co1/private.toml
}

testBuild(){
    testOK ./cosi help
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
    local D=public.toml
    shift
    echo "Running Client with $D $@"
    dbgRun ./cosi -d $DBG_APP $@ -g $D
}

runSrvCfg(){
    echo -e "localhost:200$(( 2 * $1 ))\nCosi $1\n$(pwd)/srv$1\n" | ./cosi server setup > $OUT
}

runSrv(){
    ( ./cosi -d $DBG_SRV server -c srv$1/private.toml & )

    sleep 10
}

main
