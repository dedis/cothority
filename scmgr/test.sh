#!/usr/bin/env bash

DBG_TEST=1
# Debug-level for app
DBG_APP=2
# DBG_SRV=2

. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

main(){
	startTest
	buildConode github.com/dedis/cothority/skipchain
	CFG=$BUILDDIR/config.bin
	test Restart
	test Config
	test Create
	test Join
	test Add
	test Index
	test Fetch
	test Link
	test Unlink
	test Follow
	test NewChain
	stopTest
}

testNewChain(){
	for t in none strict any; do
	  setupThree
		testOut "Starting testFollow_$t"
		testNewChain_$t
		cleanup
	done
}

testNewChain_none(){
	testFollow_id

	setupGenesis group1.toml
	testFail runSc skipchain add $ID group12.toml
}

testNewChain_strict(){
	setupGenesis group1.toml
	testOK runSc admin follow -lookup ${host[1]}:$ID ${host[2]}

	setupGenesis group1.toml
	testFail runSc skipchain add $ID group123.toml
}

testNewChain_any(){
	setupGenesis group1.toml
	testOK runSc admin follow -lookup ${host[1]}:$ID -any ${host[2]}

	setupGenesis group1.toml
	testOK runSc skipchain add $ID group123.toml
}

testFollow(){
	for t in id search lookup list delete; do
		setupThree
		testOut "Starting testFollow_$t"
		testFollow_$t
		cleanup
	done
}

setupThree(){
	startCl
	runCoBG 3
	cat co1/public.toml > group1.toml
	cat co[12]/public.toml > group12.toml
	cat co[123]/public.toml > group123.toml
	hosts=()
	for h in 1 2 3; do
		host[$h]="localhost:$(( 2000 + 2 * h ))"
		runSc admin link -priv co$h/private.toml
	done
}

testFollow_id(){
	setupGenesis group1.toml
	runSc admin follow -id 00 ${host[2]}
	testFail runSc skipchain add $ID group12.toml
	testOK runSc admin follow -id $ID ${host[2]}
	testOK runSc skipchain add $ID group12.toml
}

testFollow_search(){
	setupGenesis group1.toml
	runSc admin follow -id $ID ${host[2]}
	runSc skipchain add $ID group12.toml

	setupGenesis group1.toml
	testOK runSc admin follow -search $ID ${host[2]}
	testOK runSc skipchain add $ID group12.toml
}

testFollow_lookup(){
	setupGenesis group1.toml
	testOK runSc admin follow -lookup ${host[1]}:$ID ${host[2]}
	testOK runSc skipchain add $ID group12.toml
}

testFollow_list(){
	setupGenesis group1.toml
	runSc admin follow -lookup ${host[1]}:$ID ${host[2]}
	testGrep $ID runSc admin list ${host[2]}
}

testFollow_delete(){
	testFollow_list
	testFail runSc admin delfollow 00 ${host[2]}
	testOK runSc admin delfollow $ID ${host[2]}
	testNGrep $ID runSc admin list ${host[2]}
}

testLink(){
	startCl
	setupGenesis
	testOK [ -n "$ID" ]
	ID=""
	testFail [ -n "$ID" ]
	testOK runSc admin link -priv co1/private.toml
	testOK runSc admin follow -id 00 127.0.0.1:2002
	testFail runSc admin follow -id 00 127.0.0.1:2004
	setupGenesis
	testOK [ -n "$ID" ]
}

testUnlink(){
	startCl
	testOK runSc admin link -priv co1/private.toml
	testFail runSc admin unlink localhost:2004
	testOK runSc admin unlink localhost:2002
	testFail runSc admin unlink localhost:2002
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

testRestart(){
	startCl
	setupGenesis
	pkill -9 conode 2> /dev/null
	runCoBG 1 2
	testOK runSc sc add $ID public.toml
}

testAdd(){
	startCl
	setupGenesis
	testFail runSc sc add 1234 public.toml
	testOK runSc sc add $ID public.toml
	runCoBG 3
	runGrepSed "Latest block of" "s/.* //" runSc sc update $ID
	LATEST=$SED
	testOK runSc sc add $LATEST public.toml
}

setupGenesis(){
	runGrepSed "Created new" "s/.* //" runSc sc create ${1:-public.toml}
	ID=$SED
}

testJoin(){
	startCl
	runGrepSed "Created new" "s/.* //" runSc sc create public.toml
	ID=$SED
	rm -f $CFG
	testGrep "Didn't find any" runSc list known
	testFail runSc list join public.toml 1234
	testGrep "Didn't find any" runSc list known
	testOK runSc list join public.toml $ID
	testGrep $ID runSc list known -l
}

testCreate(){
	startCl
	testGrep "Didn't find any" runSc list known -l
	testFail runSc sc create
	testOK runSc sc create public.toml
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
	testOK runSc sc create one.toml
	testOK runSc sc create public.toml
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
