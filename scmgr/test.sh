#!/usr/bin/env bash

DBG_TEST=1
# Debug-level for app
DBG_APP=2
# DBG_SRV=2

. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

main(){
	startTest
	buildConode github.com/dedis/cothority/skipchain
	CFG=$BUILDDIR/scmgr_config
	test Restart
	test Config
	test Create
	test Join
	test Add
	test Index
	test Fetch
	test Link
	test Linklist
	test Unlink
	test Follow
	test NewChain
	test Failure
	stopTest
}

testNewChain(){
	for t in none strict any; do
		setupThree
		testOut "Starting testNewChain_$t"
		testNewChain_$t
		cleanup
	done
}

testNewChain_none(){
	testFollow_id

	setupGenesis group1.toml
	if [ -z "$ID" ]; then
		echo "id is empty"
		exit 1
	fi

	testFail runSc skipchain block add -roster group12.toml $ID
}

testNewChain_strict(){
	setupGenesis group1.toml
	testOK runSc follow add roster -lookup ${host[1]} $ID ${host[2]}

	setupGenesis group1.toml
	testFail runSc skipchain block add -roster group123.toml $ID
}

testNewChain_any(){
	setupGenesis group1.toml
	testOK runSc follow add roster -lookup ${host[1]} -any $ID ${host[2]}

	setupGenesis group1.toml
	testOK runSc skipchain block add -roster group123.toml $ID
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
		runSc link add co$h/private.toml
	done
}

testFollow_id(){
	setupGenesis group1.toml
	runSc follow add single 00 ${host[2]}
	testFail runSc skipchain block add --roster group12.toml $ID
	testOK runSc follow add single $ID ${host[2]}
	testOK runSc skipchain block add --roster group12.toml $ID
}

testFollow_search(){
	setupGenesis group1.toml
	runSc follow add single $ID ${host[2]}
	runSc skipchain block add --roster group12.toml $ID

	setupGenesis group1.toml
	testOK runSc follow add roster $ID ${host[2]}
	testOK runSc skipchain block add --roster group12.toml $ID
}

testFollow_lookup(){
	setupGenesis group1.toml
	testOK runSc follow add roster -lookup ${host[1]} $ID ${host[2]}
	testOK runSc skipchain block add --roster group12.toml $ID
}

testFollow_list(){
	setupGenesis group1.toml
	runSc follow add roster -lookup ${host[1]} $ID ${host[2]}
	testGrep $ID runSc follow list ${host[2]}
}

testFollow_delete(){
	testFollow_list
	testFail runSc follow delete 00 ${host[2]}
	testOK runSc follow delete $ID ${host[2]}
	testNGrep $ID runSc follow list ${host[2]}
}

testLink(){
	startCl
	setupGenesis
	testOK [ -n "$ID" ]
	ID=""
	testFail [ -n "$ID" ]
	testOK runSc link add co1/private.toml
	testOK runSc follow add single 00 localhost:2002
	testFail runSc follow add single 00 localhost:2004
	setupGenesis
	testOK [ -n "$ID" ]
}

testLinklist(){
	startCl
	testOK runSc link list localhost:2002
}

testUnlink(){
	startCl
	testOK runSc link add co1/private.toml
	testFail runSc link del localhost:2004
	testOK runSc link del localhost:2002
	testFail runSc link del localhost:2002
}

testFetch(){
	startCl
	setupGenesis
	rm -rf "$CFG"
	testFail runSc scdns fetch
	testOK runSc scdns fetch public.toml $ID
	testGrep 2002 runSc scdns list
	testGrep 2004 runSc scdns list
}

testRestart(){
	startCl
	setupGenesis
	pkill conode
	sleep .1
	runCoBG 1 2
	testOK runSc skipchain block add --roster public.toml $ID
}

testAdd(){
	startCl
	setupGenesis
	testFail runSc skipchain block add --roster public.toml 1234
	testOK runSc skipchain block add --roster public.toml $ID
	runCoBG 3
	testOK runSc skipchain block add --roster public.toml $ID
}

setupFour(){
	rm -f public.toml
	for n in $( seq 4 ); do
		co=co$n
		rm -f $co/*
		mkdir -p $co
		echo -e "localhost:200$(( 2 * $n ))\nCot-$n\n$co\n" | dbgRun runCo $n setup
		if [ ! -f $co/public.toml ]; then
			echo "Setup failed: file $co/public.toml is missing."
			exit
		fi
		cat $co/public.toml >> public.toml
	done
}

testFailure() {
	rm -rf "$CFG"
	setupFour
	runCoBG 1 2 3 4
	setupGenesis
	testOK runSc skipchain block add --roster public.toml $ID

	# -n: newest, so #4 is the one that is dead now
	pkill -n conode
	sleep .1
	testOK runSc skipchain block add --roster public.toml $ID

	runCoBG 4
	sleep .1
	testOK runSc skipchain block add --roster public.toml $ID
}

setupGenesis(){
	runGrepSed "Created new" "s/.* //" runSc skipchain create ${1:-public.toml}
	ID=$SED
}

testJoin(){
	startCl
	runGrepSed "Created new" "s/.* //" runSc skipchain create public.toml
	ID=$SED
	rm -rf "$CFG"
	testGrep "Didn't find any" runSc scdns list
	testFail runSc scdns fetch public.toml 1234
	testGrep "Didn't find any" runSc scdns list
	testOK runSc scdns fetch public.toml $ID
	testGrep $ID runSc scdns list -l
}

testCreate(){
	startCl
	testGrep "Didn't find any" runSc scdns list -l
	testFail runSc skipchain create
	testOK runSc skipchain create public.toml
	testGrep "Genesis-block" runSc scdns list -l
}

testIndex(){
	startCl
	setupGenesis
	touch random.js

	testFail runSc scdns index
	testOK runSc scdns index $PWD
	testGrep "$ID" cat index.js
	testGrep "localhost" cat index.js
	testGrep "$ID" cat "$ID.js"
	testGrep "localhost" cat "$ID.js"
	testNFile random.js
	dbgOut ""
}

testConfig(){
	startCl
	OLDCFG=$CFG
	CFGDIR=$( mktemp -d )
	CFG=$CFGDIR/scmgr_config
	rmdir $CFGDIR
	head -n 4 public.toml > one.toml
	testOK runSc skipchain create one.toml
	testOK runSc skipchain create public.toml
	rm -rf $CFGDIR
	CFG=$OLDCFG

	# $CFG/data cannot be empty
	testFail [ -d "$CFG/data" ]
}

runSc(){
	dbgRun ./$APP -c $CFG -d $DBG_APP $@
}

startCl(){
	rm -rf "$CFG"
	runCoBG 1 2
}

main
