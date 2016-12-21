#!/usr/bin/env bash

DBG_SHOW=2
# Debug-level for server
DBG_SRV=1
DBG_APP=1
# For easier debugging
STATICDIR=test
NBR_CLIENTS=3
NBR_SERVERS=3

. ../libcothority/cothority.sh

main(){
    startTest
    buildApp
#    test Build
#    test Cothority
#    test MgrLink
#	test Save
#    test MgrConfig
#	test ClCreate
#	test MgrPublic
#	test MgrFinal
#	test MgrFinal2
#	test ClJoin
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
	runCl 1 client sign msg1 ctx1 > sign1.toml
	runCl 2 client sign msg1 ctx1 > sign2.toml
	tag1=$( grep Tag: sign1.toml | sed -e "s/.* //")
	sig1=$( grep Signature: sign1.toml | sed -e "s/.* //")
	tag2=$( grep Tag: sign2.toml | sed -e "s/.* //")
	sig2=$( grep Signature: sign2.toml | sed -e "s/.* //")
}

testClSign(){
	mkClJoin
	testFail runCl 1 client sign
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
	runCl 1 mgr public $pub1
	runCl 1 mgr public $pub2
	runCl 2 mgr public $pub1
	runCl 2 mgr public $pub2
	runCl 1 mgr final
	runCl 2 mgr final | tail -n +3 | head -n 8 > final.toml
}

testMgrFinal2(){
	mkLink
	mkKeypair
	mkPopConfig
	runCl 1 mgr config pop_desc.toml group.toml
	runCl 2 mgr config pop_desc.toml group.toml
	runCl 1 mgr public $pub2
	runCl 2 mgr public $pub1
	runCl 2 mgr public $pub2
	testFail runCl 1 mgr final
	testOK runCl 2 mgr final
	testOK runCl 1 mgr final
	runCl 1 mgr final > final1.toml
	runCl 2 mgr final > final2.toml
	testNGrep , echo $( runCl 1 mgr final | grep Attend )
	testNGrep , echo $( runCl 2 mgr final | grep Attend )
	testOK [ $( md5 -q final1.toml ) = $( md5 -q final2.toml ) ]
}

testMgrFinal(){
	mkLink
	mkKeypair
	mkPopConfig
	runCl 1 mgr config pop_desc.toml group.toml
	runCl 2 mgr config pop_desc.toml group.toml
	runCl 1 mgr public $pub2
	runCl 2 mgr public $pub1
	runCl 2 mgr public $pub2
	testFail runCl 1 mgr final
	testOK runCl 2 mgr final
	testOK runCl 1 mgr final
	runCl 1 mgr final > final1.toml
	runCl 2 mgr final > final2.toml
	testNGrep , echo $( runCl 1 mgr final | grep Attend )
	testNGrep , echo $( runCl 2 mgr final | grep Attend )
	testOK [ $( md5 -q final1.toml ) = $( md5 -q final2.toml ) ]
}

mkConfig(){
	local cl
	mkLink
	mkKeypair
	mkPopConfig
	for cl in $@; do
		runCl $cl mgr config pop_desc.toml group.toml
	done
}

testMgrFinal(){
	mkLink
	mkKeypair
	mkPopConfig
	runCl 1 mgr config pop_desc.toml group.toml
	runCl 2 mgr config pop_desc.toml group.toml
	runCl 1 mgr public $pub1
	runCl 1 mgr public $pub2
	runCl 2 mgr public "\[\"$pub1\",\"$pub2\"\]"
	testFail runCl 1 mgr final
	testOK runCl 2 mgr final
}

testMgrPublic(){
	mkKeypair
	testFail runCl 1 mgr public
	testOK runCl 1 mgr public $pub1
	testFail runCl 1 mgr public $pub1
	testOK runCl 1 mgr public $pub2
}

testClCreate(){
	testOK runCl 1 client create
	mkKeypair
	testOK [ $( md5 -q keypair.1 ) != $( md5 -q keypair.2 ) ]
}

mkKeypair(){
	runCl 1 client create > keypair.1
	runCl 1 client create > keypair.2
	priv1=$( grep Private keypair.1 | sed -e "s/.* //" )
	priv2=$( grep Private keypair.2 | sed -e "s/.* //" )
	pub1=$( grep Public keypair.1 | sed -e "s/.* //" )
	pub2=$( grep Public keypair.2 | sed -e "s/.* //" )
}

testMgrConfig(){
	mkPopConfig
	testFail runCl 1 mgr config pop_desc.toml group.toml
	mkLink
	testOK runCl 1 mgr config pop_desc.toml group.toml
	testOK runCl 2 mgr config pop_desc.toml group.toml
}

mkPopConfig(){
	cat << EOF > pop_desc.toml
Name = "33c3 Proof-of-Personhood Party"
DateTime = "2016-12-29 15:00 UTC"
Location = "Earth, Germany, Hannover, Hall A1"
EOF
}

testSave(){
	runCoBG 1
	runCoBG 2
	testFail runCl 1 mgr config pop_desc.toml group.toml
	pkill -9 -f cothority
	mkLink
	pkill -9 -f cothority
	runCoBG 1
	runCoBG 2
	testOK runCl 1 mgr config pop_desc.toml group.toml
}

mkLink(){
	runCoBG 1
	runCoBG 2
	runCl 1 mgr link $addr1
	pin1=$( grep PIN ${COLOG}1.log | sed -e "s/.* //" )
	runCl 1 mgr link $addr1 $pin1
	runCl 2 mgr link $addr2
	pin2=$( grep PIN ${COLOG}2.log | sed -e "s/.* //" )
	runCl 2 mgr link $addr2 $pin2
}

testMgrLink(){
	runCoBG 1
	runCoBG 2
	testOK runCl 1 mgr link $addr1
	testGrep PIN cat ${COLOG}1.log
	pin1=$( grep PIN ${COLOG}1.log | sed -e "s/.* //" )
	testFail runCl 1 mgr link $addr1 abcdefg
	testOK runCl 1 mgr link $addr1 $pin1
	testOK runCl 2 mgr link $addr2
	testGrep PIN cat ${COLOG}2.log
	pin2=$( grep PIN ${COLOG}2.log | sed -e "s/.* //" )
	testOK runCl 2 mgr link $addr2 $pin2
}

testBuild(){
    testOK dbgRun ./cothority --help
    testOK dbgRun ./pop --help
}

runCl(){
    local D=cl$1
    shift
    dbgRun ./pop -d $DBG_APP -c $D $@
}

buildApp(){
    startTest
	appBuild
	cothorityBuild
    echo "Creating directories"
    for n in $(seq $NBR_CLIENTS); do
        cl=cl$n
        rm -f $cl/*
        mkdir -p $cl
    done
    addr1=127.0.0.1:2002
    addr2=127.0.0.1:2004
    addr3=127.0.0.1:2006
}

if [ "$1" -a "$STATICDIR" ]; then
    rm -f $STATICDIR/{cothority,pop}
fi

main
