#!/usr/bin/env bash

. ./libtest.sh

main(){
    startTest
    build
    test Build
    test ServerConfig
    test ClientConfig
    test ClientAdd
    test ServerAdd
    test ClientDel
    test ServerDel
    stopTest
}

testServerDel(){
    testServerAdd
    runClDbg 1 server del localhost:2001
    runClDbg 2 update
    runClDbg 2 confirm
    runClDbg 2 update
    testNGrep 2001 runCl 2 list

    testGrep 2001 runCl 1 list
    runClDbg 1 update
    testNGrep 2001 runCl 1 list
}
testClientDel(){
    testClientAdd
    runClDbg 1 clientRemove
    testGrep TestClient-cl1 runCl 2 list
    runClDbg 2 update
    runClDbg 2 confirm
    runClDbg 2 update
    testNGrep TestClient-cl1 runCl 2 list
}

testServerAdd(){
    runSrvCfgDbg 1
    runSrvCfgDbg 2
    runSrvCfgDbg 3
    sleep .2
    runClDbg 1 server add localhost:2001
    runClDbg 1 server add localhost:2002
    testGrep 2001 runCl 1 list
    testGrep 2002 runCl 1 list

    runClDbg 2 server add localhost:2001
    testNGrep 2001 runCl 2 list
    testNGrep 2002 runCl 2 list
    runClDbg 2 server propose localhost:2001
    runClDbg 1 update
    runClDbg 1 confirm
    runClDbg 2 server add localhost:2001

    runClDbg 2 server add localhost:2003
    runClDbg 1 update
    runClDbg 1 confirm
    runClDbg 1 update
    runClDbg 1 listNew
    runClDbg 1 list
    testGrep 2003 runCl 1 list
}

testClientAdd(){
    runSrvCfgDbg 1
    sleep .2
    runClDbg 1 server add localhost:2001
    sleep .2
    testGrep TestClient-cl1 runCl 1 list
    runClDbg 2 server add localhost:2001
    testGrep TestClient-cl2 runCl 2 list
    testNGrep TestClient-cl1 runCl 2 list

    runClDbg 2 server propose localhost:2001
    runClDbg 1 update
    testGrep TestClient-cl2 runCl 1 listNew
    #runClDbg 2 update
    #testNGrep TestClient-cl1 runCl 2 list
    runClDbg 1 confirm
    testGrep TestClient-cl2 runCl 1 list

    runClDbg 2 server add localhost:2001
    runClDbg 2 update
    testGrep TestClient-cl1 runCl 2 list
}

testClientConfig(){
    runClDbg 1 list &
    runClDbg 2 list &
    sleep 1
    testFile cl1/config.bin
    testFile cl2/config.bin
    pkill -f ssh-ksc
}

testServerConfig(){
    runSrvCfgDbg 1
    runSrvCfgDbg 2
    sleep 1
    testOK lsof -n -i:2001
    testOK lsof -n -i:2002
    pkill -f ssh-kss
    testFile srv1/config.bin
    testFile srv2/config.bin
}

testBuild(){
    echo "Testing build"
    testOK ./ssh-kss help
    testOK ./ssh-ksc -c cl1 -cs cl1 help
}

runCl(){
    D=cl$1
    shift
    dbgRun "Running Client with $D $@"
    ./ssh-ksc -d $DBG_CLIENT -c $D --cs $D $@
}

runClDbg(){
    runCl $@ >> $OUT
}

runSrvCfg(){
    echo -e "localhost:200$1\nsrv$1\nsrv$1\n" | runSrv $1
}

runSrv(){
    ./ssh-kss -d $DBG_SRV -c srv$1/config.bin
}

runSrvCfgDbg(){
    runSrvCfg $@ >> $OUT 2> /dev/null &
}

runSrvDbg(){
    runSrv $@ >> $OUT
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
    for app in ssh-ksc ssh-kss; do
        if [ ! -e $app -o "$BUILD" ]; then
            if ! go build $BUILDDIR/$app/$app.go; then
                fail "Couldn't build $app"
            fi
        fi
    done
    echo "Creating keys"
    for n in $(seq $NBR); do
        srv=srv$n
        if [ -d $srv ]; then
            rm -f $srv/*bin
        else
            mkdir $srv
            ssh-keygen -t rsa -b 4096 -N "" -f $srv/ssh_host_rsa_key > /dev/null
        fi
        cl=cl$n
        if [ -d $cl ]; then
            rm -f $cl/*bin
        else
            mkdir $cl
            ssh-keygen -t rsa -b 4096 -N "" -f $cl/id_rsa > /dev/null
        fi
    done
}

main
