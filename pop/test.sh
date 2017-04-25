#!/usr/bin/env bash

DBG_TEST=1
DBG_APP=3
NBR_CLIENTS=3
NBR_SERVERS=3

. $GOPATH/src/gopkg.in/dedis/onet.v1/app/libtest.sh

main(){
	startTest
	COT=github.com/dedis/cothority
	buildConode $COT/cosi/service $COT/pop/service
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
	test OrgPublic
	test OrgFinal1
	test OrgFinal2
	test OrgFinal3
	test ClJoin
	test ClSign
	test ClVerify
	stopTest
}

testClVerify(){
	mkClSign
	testFail runCl 1 client verify msg1 ctx1 $tag1 $sig1
	testOK runCl 1 client verify msg1 ctx1 $sig1 $tag1
	testFail runCl 1 client verify msg1 ctx1 $sig1 $tag2
	testOK runCl 1 client verify msg1 ctx1 $sig2 $tag2
	testFail runCl 1 client verify msg1 ctx1 $sig2 $tag1
}

mkClSign(){
	mkClJoin
	runDbgCl 1 1 client sign msg1 ctx1 > sign1.toml
	runDbgCl 1 2 client sign msg1 ctx1 > sign2.toml
	tag1=$( grep Tag: sign1.toml | sed -e "s/.* //")
	sig1=$( grep Signature: sign1.toml | sed -e "s/.* //")
	tag2=$( grep Tag: sign2.toml | sed -e "s/.* //")
	sig2=$( grep Signature: sign2.toml | sed -e "s/.* //")
}

testClSign(){
	mkClJoin
	testFail runCl 1 client sign
	testOK runCl 1 client sign msg1 ctx1
	testOK runCl 1 client sign msg1 ctx1
}

mkClJoin(){
	mkFinal
	runCl 1 client join final.toml $priv1
	runCl 2 client join final.toml $priv2
}

testClJoin(){
	mkFinal
	testFail runCl 1 client join final.toml
	testFail runCl 1 client join final.toml badkey
	testOK runCl 1 client join final.toml $priv1
	testOK runCl 2 client join final.toml $priv2
}

mkFinal(){
	mkConfig 1 2
	runCl 1 org public $pub1
	runCl 1 org public $pub2
	runCl 2 org public $pub1
	runCl 2 org public $pub2
	runCl 1 org final
	runDbgCl 1 2 org final | tail -n +3 | head -n 8 > final.toml
}

testOrgFinal3(){
	mkLink
	mkKeypair
	mkPopConfig
	runCl 1 org config pop_desc.toml public.toml
	runCl 2 org config pop_desc.toml public.toml
	runCl 1 org public $pub2
	runCl 2 org public $pub1
	runCl 2 org public $pub2
	testFail runCl 1 org final
	testOK runCl 2 org final
	testOK runCl 1 org final
	runDbgCl 1 1 org final > final1.toml
	runDbgCl 1 2 org final > final2.toml
	testNGrep , echo $( runCl 1 org final | grep Attend )
	testNGrep , echo $( runCl 2 org final | grep Attend )
	testOK [ $( md5 -q final1.toml ) = $( md5 -q final2.toml ) ]
}

testOrgFinal2(){
	mkLink
	mkKeypair
	mkPopConfig
	runCl 1 org config pop_desc.toml public.toml
	runCl 2 org config pop_desc.toml public.toml
	runCl 1 org public $pub2
	runCl 2 org public $pub1
	runCl 2 org public $pub2
	testFail runCl 1 org final
	testOK runCl 2 org final
	testOK runCl 1 org final
	runDbgCl 1 1 org final > final1.toml
	runDbgCl 1 2 org final > final2.toml
	testNGrep , echo $( runCl 1 org final | grep Attend )
	testNGrep , echo $( runCl 2 org final | grep Attend )
	testOK [ $( md5 -q final1.toml ) = $( md5 -q final2.toml ) ]
}

mkConfig(){
	local cl
	mkLink
	mkKeypair
	mkPopConfig
	for cl in $@; do
		runCl $cl org config pop_desc.toml public.toml
	done
}

testOrgFinal1(){
	mkLink
	mkKeypair
	mkPopConfig
	runCl 1 org config pop_desc.toml public.toml
	runCl 2 org config pop_desc.toml public.toml
	runCl 1 org public $pub1
	runCl 1 org public $pub2
	runCl 2 org public "\[\"$pub1\",\"$pub2\"\]"
	testFail runCl 1 org final
	testOK runCl 2 org final
}

testOrgPublic(){
	mkKeypair
	testFail runCl 1 org public
	testOK runCl 1 org public $pub1
	testFail runCl 1 org public $pub1
	testOK runCl 1 org public $pub2
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
