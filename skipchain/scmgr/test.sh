#!/usr/bin/env bash

DBG_TEST=2
# Debug-level for app
DBG_APP=2
#DBG_SRV=3

. $GOPATH/src/gopkg.in/dedis/onet.v1/app/libtest.sh

main(){
	startTest
	buildConode github.com/dedis/cothority/skipchain/service
	CFG=$BUILDDIR/config.bin
	test Restart
	test Config
	test Create
	test Join
	test Add
	test Index
	test Html
	test Fetch
	stopTest
}

testFetch(){
	startCl
	setupGenesis
	rm -f $CFG
	testFail runSc list fetch
	testOK runSc list fetch public.toml
	testGrep 2002 runSc list known
	testGrep 2004 runSc list known
}

testHtml(){
	startCl
	testOK runSc create -url http://dedis.ch public.toml
	ID=$( runSc list known | head -n 1 | sed -e "s/.*SkipChain \(.*\)/\1/" )
	html=$(mktemp)
	echo "TestWeb" > $html
	testOK runSc addWeb $ID $html
	rm -f $html
}

testRestart(){
	startCl
	setupGenesis
	pkill -9 conode 2> /dev/null
	runCoBG 1 2
	testOK runSc add $ID public.toml
}

testAdd(){
	startCl
	setupGenesis
	testFail runSc add 1234 public.toml
	testOK runSc add $ID public.toml
	runCoBG 3
	runGrepSed "Latest block of" "s/.* //" runSc update $ID
	testOK runSc update -d $ID
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
	rm -f $CFG
	testGrep "Didn't find any" runSc list known
	testFail runSc join public.toml 1234
	testGrep "Didn't find any" runSc list known
	testOK runSc join public.toml $ID
	testGrep $ID runSc list known -l
}

testCreate(){
	startCl
	testGrep "Didn't find any" runSc list known -l
	testFail runSc create
	testOK runSc create public.toml
	testGrep "Genesis-block" runSc list known -l
}

testIndex(){
	startCl
	setupGenesis
	touch random.html

	testFail runSc list index
	testOK runSc list index $PWD
	testGrep "$ID" cat index.html
	testGrep "127.0.0.1" cat index.html
	testGrep "$ID" cat "$ID.html"
	testGrep "127.0.0.1" cat "$ID.html"
	testNFile random.html
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
	rm -f $CFG
	runCoBG 1 2
}

main
