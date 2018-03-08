#!/usr/bin/env bash

DBG_TEST=2
# Debug-level for app
DBG_APP=2
DBG_SRV=2

# Have always 3 servers
NBR=3
NBR_SERVERS_GROUP=3

. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

main(){
    startTest
    mkdir -p cl{1..3}
    buildConode
    test Build
    test Create
    test ManageJoin
    #test Write
    #test Read
    test SCRead
    stopTest
}

testSCRead(){
	setupOCS
	testGrep last runCl 1 skipchain
    runGrepSed "Stored file" "s/.* //" runCl 1 write public.toml $READER1_PUB
    FILE=$SED
    runGrepSed "Next block:" "s/.* //" runCl 1 skipchain
    NEXT=$SED
    testOK runCl 1 skipchain $NEXT
}

testRead(){
    setupOCS
    runGrepSed "Stored file" "s/.* //" runCl 1 write public.toml $READER1_PUB
    FILE=$SED
    testOK runCl 2 manage join public.toml $SID
    testFail runCl 2 read $FILE $READER2_PRIV
    tmp=$( mktemp )
    testOK runCl 2 read -o $tmp $FILE $READER1_PRIV
    testOK cmp public.toml $tmp
    rm $tmp
}

testWrite(){
    setupOCS
    testFail runCl 1 write public.toml
    testOK runCl 1 write public.toml $READER1_PUB
    testOK runCl 2 manage join public.toml $SID
    testOK runCl 2 write public.toml $READER2_PUB
}

testManageJoin(){
    setupOCS
    testOK runCl 2 manage join public.toml $SID
}

setupOCS(){
    runCoBG 1 2 3
    runGrepSed skipchainid "s/.* //" runCl 1 manage create public.toml
    SID=$SED
    READER1=$( runCl 1 keypair )
    READER1_PRIV=$( echo $READER1 | cut -f 1 -d : )
    READER1_PUB=$( echo $READER1 | cut -f 2 -d : )
    READER2=$( runCl 1 keypair )
    READER2_PRIV=$( echo $READER2 | cut -f 1 -d : )
    READER2_PUB=$( echo $READER2 | cut -f 2 -d : )
}

testCreate(){
    runCoBG 1 2 3
    testOK runCl 1 manage create public.toml
}

testBuild(){
    testOK dbgRun runCl 1 --help
}

runCl(){
    local D=cl$1
    shift
    dbgRun ./$APP -d $DBG_APP -c $D $@
}

main
