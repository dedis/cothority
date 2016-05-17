#!/usr/bin/env bash

. ./libtest.sh

tails=8

main(){
    startTest
    build
    test Build
    test ServerCfg
    test SignFile
    stopTest
}

testSignFile(){
    setupServers 1
    echo $OUT
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
    runSrvCfg 1
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
    rm -f srv1/*
    rm -f srv2/*
    runSrvCfg 1 
    tail -n 4 srv1/group.toml >  $SERVERS
    runSrvCfg 2 
    echo >> $SERVERS
    tail -n 4 srv2/group.toml >> $SERVERS
    runSrv 1 &
    runSrv 2 &
    OUT=$OOUT
}

runCl(){
    D=cl$1/servers.toml
    shift
    echo "Running Client with $D $@"
    ./cosi -d 3 -g $D $@
}

runSrvCfg(){
    echo -e "200$1\n127.0.0.1:200$1\n$(pwd)/srv$1/config.toml\n" | ./cothorityd setup > $OUT
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

main
