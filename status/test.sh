#!/usr/bin/env bash

DBG_TEST=1
DBG_APP=2

. ../libtest.sh

main(){
    startTest
    buildConode go.dedis.ch/cothority/v3/status/service
    run testBuild
    run testNetwork
    run testConnectivity
    stopTest
}

testNetwork(){
    runCoBG 1 2
    testOut "Running network"
    testGrep "Available_Services" runCl -g public.toml
    testGrep "Available_Services" runCl -g public.toml
    testGrep "Available_Services" runCl --host localhost:2002
    testGrep "Available_Services" runCl --host tls://localhost:2002
}

testConnectivity(){
    runCoBG 1 2 3
    testOK runCl connectivity public.toml co1/private.toml
    cat co3/public.toml >> public.toml
    testOK runCl connectivity public.toml co1/private.toml
    pkill -f conode.*co2
    testFail runCl connectivity public.toml co1/private.toml
    testGrep "The following nodes" runCl connectivity public.toml co1/private.toml --findFaulty
    grep -v "List is" $RUNOUT > /tmp/runout
    mv /tmp/runout $RUNOUT
    testReGrep 2002
    testReNGrep 2004
    testReGrep 2006
}

testBuild(){
    testOK runCl --help
    testOK runCo 1 --help
}

runCl(){
    dbgRun ./status -d $DBG_APP $@
}

main
