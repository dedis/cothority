#!/usr/bin/env bash

DBG_TEST=1
DBG_APP=3
NBR_CLIENTS=3
NBR_SERVERS=3

. $GOPATH/src/gopkg.in/dedis/onet.v1/app/libtest.sh

main(){
    startTest
    buildConode
    echo "Creating directories"
    for n in $(seq $NBR_CLIENTS); do
        cl=cl$n
        rm -f $cl/*
        mkdir -p $cl
    done
    addr1=127.0.0.1:2002
    addr2=127.0.0.1:2004
    addr3=127.0.0.1:2006

    test Build
    test Check
    test OrgLink
	test Save
    test OrgConfig
	test ClCreate
    stopTest
}

testClCreate(){
	testOK runCl 1 client create
	mkKeypair
	testOK [ $( md5 -q keypair.1 ) != $( md5 -q keypair.2 ) ]
}

mkKeypair(){
	runDbgCl 1 1 client create > keypair.1
	runDbgCl 1 1 client create > keypair.2
	priv1=$( grep Private keypair.1 | sed -e "s/.* //" )
	priv2=$( grep Private keypair.2 | sed -e "s/.* //" )
	pub1=$( grep Public keypair.1 | sed -e "s/.* //" )
	pub2=$( grep Public keypair.2 | sed -e "s/.* //" )
}

testOrgConfig(){
	mkPopConfig
	testFail runCl 1 org config pop_desc.toml public.toml
	mkLink
	testOK runCl 1 org config pop_desc.toml public.toml
	testOK runCl 2 org config pop_desc.toml public.toml
}

mkPopConfig(){
	cat << EOF > pop_desc.toml
Name = "33c3 Proof-of-Personhood Party"
DateTime = "2016-12-29 15:00 UTC"
Location = "Earth, Germany, Hamburg, Hall A1"
EOF
}

testSave(){
	runCoBG 1 2
	mkPopConfig

	testFail runCl 1 org config pop_desc.toml public.toml
	pkill -9 -f conode
	mkLink
	pkill -9 -f conode
	runCoBG 1 2
	testOK runCl 1 org config pop_desc.toml public.toml
}

mkLink(){
	runCoBG 1 2
	runCl 1 org link $addr1
	pin1=$( grep PIN ${COLOG}1.log | sed -e "s/.* //" )
	runCl 1 org link $addr1 $pin1
	runCl 2 org link $addr2
	pin2=$( grep PIN ${COLOG}2.log | sed -e "s/.* //" )
	runCl 2 org link $addr2 $pin2
}

testOrgLink(){
	runCoBG 1 2
	testOK runCl 1 org link $addr1
	testGrep PIN cat ${COLOG}1.log
	pin1=$( grep PIN ${COLOG}1.log | sed -e "s/.* //" )
	testFail runCl 1 org link $addr1 abcdefg
	testOK runCl 1 org link $addr1 $pin1
	testOK runCl 2 org link $addr2
	testGrep PIN cat ${COLOG}2.log
	pin2=$( grep PIN ${COLOG}2.log | sed -e "s/.* //" )
	testOK runCl 2 org link $addr2 $pin2
}

testCheck(){
	runCoBG 1 2 3
	cat co*/public.toml > check.toml
	testOK dbgRun ./pop -d $DBG_APP check check.toml
}

testBuild(){
    testOK dbgRun ./conode --help
    testOK dbgRun ./pop --help
}

runCl(){
    local CFG=cl$1
    shift
    dbgRun ./pop -d $DBG_APP -c $CFG $@
}

runDbgCl(){
	local DBG=$1
	local CFG=cl$2
	shift 2
	./pop -d $DBG -c $CFG $@
}

main
