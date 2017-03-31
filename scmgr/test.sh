#!/usr/bin/env bash

DBG_TEST=2
# Debug-level for app
DBG_APP=2
#DBG_SRV=3

. $GOPATH/src/gopkg.in/dedis/onet.v1/app/libtest.sh

main(){
    startTest
    buildConode github.com/dedis/cothority/skipchain
    CFG=$BUILDDIR/config.bin
    test Config
	test Create
	test Join
	test Add
	test Index
    stopTest
}

testAdd(){
	startCl
	setupGenesis
	testFail runSc add 1234 public.toml
	testOK runSc add $ID public.toml
	runCoBG 3
	runGrepSed "Latest block of" "s/.* //" runSc update $ID
	LATEST=$SED
	testOK runSc add $LATEST public.toml
}

setupGenesis(){
	runGrepSed "Created new" "s/.* //" runSc create public.toml
	ID=$SED
}

testJoin(){
	startCl
	runGrepSed "Created new" "s/.* //" runSc create public.toml
	ID=$SED
	rm $CFG
	testGrep "Didn't find any" runSc list
	testFail runSc join public.toml 1234
	testGrep "Didn't find any" runSc list
	testOK runSc join public.toml $ID
	testGrep $ID runSc list -l
}

testCreate(){
	startCl
    testGrep "Didn't find any" runSc list -l
    testFail runSc create
    testOK runSc create public.toml
    testGrep "Genesis-block" runSc list -l
}

testIndex(){
    startCl
    setupGenesis
    touch random.html

    testFail runSc index
    testOK runSc index $PWD
    testGrep "$ID" cat index.html
    testGrep "127.0.0.1" cat index.html
    testGrep "$ID" cat "$ID.html"
    testGrep "127.0.0.1" cat "$ID.html"
    testFail [ -f random.html ]
}

testConfig(){
	startCl
	OLDCFG=$CFG
	CFGDIR=$( mktemp -d )
	CFG=$CFGDIR/config.bin
	rmdir $CFGDIR
	head -n 4 public.toml > one.toml
	testOK runSc create one.toml
	testOK runSc create public.toml
	rm -rf $CFGDIR
	CFG=$OLDCFG
}

runSc(){
    dbgRun ./$APP -c $CFG -d $DBG_APP $@
}

startCl(){
	rm $CFG
	runCoBG 1 2
}

main
