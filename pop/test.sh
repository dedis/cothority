#!/usr/bin/env bash

DBG_TEST=1
DBG_APP=2
# DBG_SRV=2

NBR_CLIENTS=4
NBR_SERVERS=3
NBR_SERVERS_GROUP=$NBR_SERVERS
. $(go env GOPATH)/src/github.com/dedis/onet/app/libtest.sh

MERGE_FILE=""
main(){
	startTest
	buildConode github.com/dedis/cothority/cosi/service github.com/dedis/cothority/pop/service
	echo "Creating directories"
	for n in $(seq $NBR_CLIENTS); do
		cl=cl$n
		rm -f $cl/*
		mkdir -p $cl
	done
	addr=()
	addr[1]=localhost:2002
	addr[2]=localhost:2004
	addr[3]=localhost:2006

	test Build
	test Check
	test OrgLink
	test Save
	test OrgConfig
	test AtCreate
	test OrgPublic
	test OrgPublic2
	test OrgFinal1
	test OrgFinal2
	test OrgFinal3
	test AtJoin
	test AtSign
	test AuthStore
	test AtVerify
	test AtMultipleKey
	test Merge
	test PropagateConfig
	stopTest
}

testPropagateConfig(){
	mkPopConfig 1 2
	mkLink 2
	runDbgCl 2 1 org config pop_desc1.toml > pop_hash_file
	pop_hash=$(grep config: pop_hash_file | sed -e "s/.* //")
	runDbgCl 0 1 org proposed -quiet ${addr[2]} > proposed.toml
	testGrep "City1" cat proposed.toml
	testOK cmp -s pop_desc1.toml proposed.toml
	testNGrep "City1" runCl 1 org proposed ${addr[2]}
	testNGrep "City1" runCl 1 org proposed ${addr[1]}
}

testMerge(){
	MERGE_FILE="pop_merge.toml"
	mkConfig 3 3 2 4

	# att1 - p1, p2; att2 - p2; att3 - p3;
	runCl 1 org public ${pub[1]} ${pop_hash[1]}
	runCl 2 org public ${pub[1]} ${pop_hash[1]}
	runCl 2 org public ${pub[2]} ${pop_hash[2]}
	runCl 3 org public ${pub[2]} ${pop_hash[2]}
	runCl 3 org public ${pub[3]} ${pop_hash[3]}
	runCl 1 org public ${pub[3]} ${pop_hash[3]}

	runCl 1 org public ${pub[1]} ${pop_hash[3]}
	runCl 3 org public ${pub[1]} ${pop_hash[3]}

	runCl 1 org public ${pub[4]} ${pop_hash[3]}
	runCl 3 org public ${pub[4]} ${pop_hash[3]}

	runCl 1 org final  ${pop_hash[1]}
	runDbgCl 1 2 org final  ${pop_hash[1]} | tail -n +3 > final1.toml
	runCl 2 org final  ${pop_hash[2]}
	runDbgCl 1 3 org final  ${pop_hash[2]} | tail -n +3> final2.toml
	runCl 3 org final  ${pop_hash[3]}
	runDbgCl 1 1 org final  ${pop_hash[3]} | tail -n +3 > final3.toml


	testFail runCl 1 attendee join -y ${priv[1]} final1.toml
	testFail runCl 2 attendee join -y ${priv[2]} final2.toml
	testFail runCl 3 attendee join -y ${priv[3]} final3.toml
	testFail runCl 4 attendee join -y ${priv[4]} final3.toml

	testFail runCl 1 org merge
	testFail runCl 3 org merge ${pop_hash[1]}

	testOK runCl 1 org merge ${pop_hash[1]}
	runDbgCl 1 2 org merge ${pop_hash[2]} | tail -n +3 > merge_final.toml
	for i in {1..4}
	do
		testOK runCl $i attendee join -y ${priv[$i]} merge_final.toml
	done
	runDbgCl 2 1 attendee join -y ${priv[1]} merge_final.toml > pop_hash_file
	merged_hash=$(grep hash: pop_hash_file | sed -e "s/.* //")

	for i in {1..4}; do
		runDbgCl 2 $i attendee sign msg1 ctx1 $merged_hash > sign$i.toml
		tag[$i]=$( grep Tag: sign$i.toml | sed -e "s/.* //")
		sig[$i]=$( grep Signature: sign$i.toml | sed -e "s/.* //")
	done


	for i in {1..4}; do
		for j in {1..4}; do
			testOK runCl $i attendee verify msg1 ctx1 ${sig[$j]} ${tag[$j]} $merged_hash
		done
	done
}

testAtMultipleKey(){
	mkConfig 2 2 2 3
	# att1.k1 - p1 att1.k2 - p2 att2 - p2
	runCl 1 org public ${pub[1]} ${pop_hash[1]}
	runCl 2 org public ${pub[1]} ${pop_hash[1]}

	runCl 1 org public ${pub[2]} ${pop_hash[2]}
	runCl 2 org public ${pub[2]} ${pop_hash[2]}

	runCl 1 org public ${pub[3]} ${pop_hash[2]}
	runCl 2 org public ${pub[3]} ${pop_hash[2]}

	runCl 1 org final  ${pop_hash[1]}
	runDbgCl 2 2 org final  ${pop_hash[1]} | tail -n +3 > final1.toml
	runCl 1 org final  ${pop_hash[2]}
	runDbgCl 2 2 org final  ${pop_hash[2]} | tail -n +3 > final2.toml


	testOK runCl 1 attendee join -y ${priv[1]} final1.toml
	testOK runCl 1 attendee join -y ${priv[2]} final2.toml
	testOK runCl 2 attendee join -y ${priv[3]} final2.toml

	runDbgCl 2 1 attendee sign msg1 ctx1 ${pop_hash[1]} > sign.toml
	tag[1]=$( grep Tag: sign.toml | sed -e "s/.* //")
	sig[1]=$( grep Signature: sign.toml | sed -e "s/.* //")


	runDbgCl 2 1 attendee sign msg1 ctx1 ${pop_hash[2]} > sign.toml
	tag[2]=$( grep Tag: sign.toml | sed -e "s/.* //")
	sig[2]=$( grep Signature: sign.toml | sed -e "s/.* //")


	runDbgCl 2 2 attendee sign msg1 ctx1 ${pop_hash[2]} > sign.toml
	tag[3]=$( grep Tag: sign.toml | sed -e "s/.* //")
	sig[3]=$( grep Signature: sign.toml | sed -e "s/.* //")

	testOK runCl 1 attendee verify msg1 ctx1 ${sig[1]} ${tag[1]} ${pop_hash[1]}
	testOK runCl 1 attendee verify msg1 ctx1 ${sig[2]} ${tag[2]} ${pop_hash[2]}
	testOK runCl 2 attendee verify msg1 ctx1 ${sig[3]} ${tag[3]} ${pop_hash[2]}

	testFail runCl 1 attendee verify msg1 ctx1 ${sig[2]} ${tag[2]} ${pop_hash[1]}
	testFail runCl 1 attendee verify msg1 ctx1 ${sig[1]} ${tag[1]} ${pop_hash[2]}
	testFail runCl 2 attendee verify msg1 ctx1 ${sig[2]} ${tag[2]} ${pop_hash[1]}
	testOK runCl 1 attendee verify msg1 ctx1 ${sig[3]} ${tag[3]} ${pop_hash[2]}
	testOK runCl 2 attendee verify msg1 ctx1 ${sig[2]} ${tag[2]} ${pop_hash[2]}
}

testAtVerify(){
	mkClSign
	testFail runCl 1 attendee verify msg1 ctx1 ${tag[1]} ${sig[1]}
	testFail runCl 1 attendee verify msg1 ctx1 ${tag[1]} ${sig[1]} ${pop_hash[1]}
	testFail runCl 1 attendee verify msg1 ctx1 ${sig[1]} ${tag[1]} ${pop_hash[2]}
	testOK runCl 1 attendee verify msg1 ctx1 ${sig[1]} ${tag[1]} ${pop_hash[1]}
	testFail runCl 1 attendee verify msg1 ctx1 ${sig[1]} ${tag[2]} ${pop_hash[1]}

	testFail runCl 1 attendee verify msg1 ctx1 ${sig[2]} ${tag[2]} ${pop_hash[1]}
	testOK runCl 2 attendee verify msg1 ctx1 ${sig[2]} ${tag[2]} ${pop_hash[2]}
	testFail runCl 2 attendee verify msg1 ctx1 ${sig[3]} ${tag[3]} ${pop_hash[2]}
	testOK runCl 3 attendee verify msg1 ctx1 ${sig[3]} ${tag[3]} ${pop_hash[3]}

	testOK runCl 1 attendee verify msg1 ctx1 ${sig[3]} ${tag[3]} ${pop_hash[3]}
}

testAuthStore(){
	mkFinal
	testFail runCl 1 auth store
	testOK runCl 1 auth store final1.toml
	testGrep "Stored" echo $(runDbgCl 1 1 auth store final1.toml)
}

tag=()
sig=()
mkClSign(){
	mkAtJoin
	for i in {1..3}; do
		runDbgCl 2 $i attendee sign msg1 ctx1 ${pop_hash[$i]} > sign$i.toml
		tag[$i]=$( grep Tag: sign$i.toml | sed -e "s/.* //")
		sig[$i]=$( grep Signature: sign$i.toml | sed -e "s/.* //")
	done
}

testAtSign(){
	mkFinal
	testFail runCl 1 attendee sign msg1 ctx1 ${pop_hash[1]}
	for i in {1..3}; do
		runDbgCl 2 $i attendee join -y ${priv[$i]} final$i.toml > pop_hash_file
		pop_hash[$i]=$(grep hash: pop_hash_file | sed -e "s/.* //")
	done
	testFail runCl 1 attendee sign
	testFail runCl 1 attendee sign msg1 ctx1 ${pop_hash[2]}
	testOK runCl 1 attendee sign msg1 ctx1 ${pop_hash[1]}
	testOK runCl 2 attendee sign msg2 ctx2 ${pop_hash[2]}
	testOK runCl 3 attendee sign msg3 ctx3 ${pop_hash[3]}
}

mkAtJoin(){
	mkFinal
	for i in {1..3}; do
		runCl $i attendee join -y ${priv[$i]} final$i.toml
	done
}

testAtJoin(){
	mkConfig 3 3 2 3

	# att1 - p1, p2; att2 - p2; att3 - p3;
	runCl 1 org public ${pub[1]} ${pop_hash[1]}
	runCl 2 org public ${pub[1]} ${pop_hash[1]}
	runCl 2 org public ${pub[2]} ${pop_hash[2]}
	runCl 3 org public ${pub[2]} ${pop_hash[2]}
	runCl 3 org public ${pub[3]} ${pop_hash[3]}
	runCl 1 org public ${pub[3]} ${pop_hash[3]}

	runCl 2 org public ${pub[1]} ${pop_hash[2]}
	runCl 3 org public ${pub[1]} ${pop_hash[2]}

	# check that fails without finalization
	testFail runCl 1 attendee join -y ${priv[1]} ${pop_hash[1]}

	runCl 1 org final  ${pop_hash[1]}
	runDbgCl 2 2 org final  ${pop_hash[1]} | tail > final1.toml
	runCl 2 org final  ${pop_hash[2]}
	runDbgCl 2 3 org final  ${pop_hash[2]} | tail > final2.toml
	runCl 3 org final  ${pop_hash[3]}
	runDbgCl 2 1 org final  ${pop_hash[3]} | tail > final3.toml

	testFail runCl 1 attendee join -y
	testFail runCl 1 attendee join -y ${priv[1]}
	testFail runCl 1 attendee join -y badkey final1.toml
	testFail runCl 1 attendee join -y ${priv[1]} final3.toml
	testOK runCl 1 attendee join -y ${priv[1]} final1.toml
	testOK runCl 2 attendee join -y ${priv[2]} final2.toml
	testOK runCl 3 attendee join -y ${priv[3]} final3.toml
	runDbgCl 2 1 attendee join -y ${priv[1]} final1.toml > tmp_file
	testGrep "hash" cat tmp_file
}

mkFinal(){
	mkConfig 3 3 2 3

	# att1 - p1, p2; att2 - p2; att3 - p3;
	runCl 1 org public ${pub[1]} ${pop_hash[1]}
	runCl 2 org public ${pub[1]} ${pop_hash[1]}
	runCl 2 org public ${pub[2]} ${pop_hash[2]}
	runCl 3 org public ${pub[2]} ${pop_hash[2]}
	runCl 3 org public ${pub[3]} ${pop_hash[3]}
	runCl 1 org public ${pub[3]} ${pop_hash[3]}

	runCl 1 org public ${pub[1]} ${pop_hash[3]}
	runCl 3 org public ${pub[1]} ${pop_hash[3]}

	runCl 1 org final  ${pop_hash[1]}
	runDbgCl 2 2 org final  ${pop_hash[1]} | tail -n +3 > final1.toml
	runCl 2 org final  ${pop_hash[2]}
	runDbgCl 2 3 org final  ${pop_hash[2]} | tail -n +3> final2.toml
	runCl 3 org final  ${pop_hash[3]}
	runDbgCl 2 1 org final  ${pop_hash[3]} | tail -n +3 > final3.toml
}

testOrgFinal3(){
	mkConfig 3 3 2 1
	runCl 1 org public ${pub[1]} ${pop_hash[1]}
	runCl 2 org public ${pub[1]} ${pop_hash[1]}
	runCl 2 org public ${pub[1]} ${pop_hash[2]}
	runCl 3 org public ${pub[1]} ${pop_hash[2]}
	runCl 3 org public ${pub[1]} ${pop_hash[3]}
	runCl 1 org public ${pub[1]} ${pop_hash[3]}

	testFail runCl 1 org final ${pop_hash[1]}
	testFail runCl 3 org final ${pop_hash[1]}
	testOK runCl 2 org final ${pop_hash[1]}

	testFail runCl 2 org final ${pop_hash[2]}
	testOK runCl 3 org final ${pop_hash[2]}

	testFail runCl 1 org final ${pop_hash[3]}
	testOK runCl 3 org final ${pop_hash[3]}
}


testOrgFinal2(){
	mkConfig 2 1 1 2
	runCl 1 org public ${pub[2]} ${pop_hash[1]}
	runCl 2 org public ${pub[1]} ${pop_hash[1]}
	runCl 2 org public ${pub[2]} ${pop_hash[1]}
	testFail runCl 1 org final ${pop_hash[1]}
	testOK runCl 2 org final ${pop_hash[1]}
	testOK runCl 1 org final ${pop_hash[1]}
	runDbgCl 1 1 org final ${pop_hash[1]} > final1.toml
	runDbgCl 1 2 org final ${pop_hash[1]} > final2.toml
	testNGrep , echo $( runCl 1 org final | grep Attend )
	testNGrep , echo $( runCl 2 org final | grep Attend )
	cmp -s final1.toml final2.toml
	testOK [ $? -eq 0 ]
}

testOrgFinal1(){
	mkConfig 2 1 1 2
	runCl 1 org public ${pub[1]} ${pop_hash[1]}
	runCl 1 org public ${pub[2]} ${pop_hash[1]}
	runCl 2 org public "\[\"${pub[1]}\",\"${pub[2]}\"\]" ${pop_hash[1]}
	testFail runCl 1 org final
	testFail runCl 1 org final bad_hash
	testFail runCl 1 org final ${pop_hash[1]}
	testOK runCl 2 org final ${pop_hash[1]}
}

testOrgPublic2(){
	mkConfig 3 3 2 1
	testOK runCl 1 org public ${pub[1]} ${pop_hash[1]}
	testOK runCl 2 org public ${pub[1]} ${pop_hash[1]}
	testOK runCl 2 org public ${pub[1]} ${pop_hash[2]}
	testOK runCl 3 org public ${pub[1]} ${pop_hash[2]}
	testOK runCl 3 org public ${pub[1]} ${pop_hash[3]}
	testOK runCl 1 org public ${pub[1]} ${pop_hash[3]}

	testFail runCl 3 org public ${pub[1]} ${pop_hash[2]}
}

testOrgPublic(){
	mkConfig 1 1 1 2
	testFail runCl 1 org public
	testFail runCl 1 org public ${pub[1]}
	testFail runCl 1 org public ${pub[1]} wrong_hash
	testOK runCl 1 org public ${pub[1]} ${pop_hash[1]}
	testFail runCl 1 org public ${pub[1]} ${pop_hash[1]}
	testOK runCl 1 org public ${pub[2]} ${pop_hash[1]}
}

# need to store many party hashes as variables
pop_hash=()
# usage: $1 organizers(conodes) and $2 parties, each node has $3 parties, $4 attendees
# example: 3 organizers, 2 parties for each
# 1st node: parties #1, #2
# 2nd node: parties #2, #3
# 3rd node: parties #1, #3
mkConfig(){
	local cl
	local pc
	mkLink $1
	mkPopConfig $2 $1
	mkKeypair $4
	for (( cl=1; cl<=$1; cl++ ))
	do
		for (( pc=1; pc<=$3; pc++ ))
		do
			num_pc=$((($pc + $cl) % $2 + 1))
			runDbgCl 2 $cl org config pop_desc$num_pc.toml $MERGE_FILE > pop_hash_file
			pop_hash[$num_pc]=$(grep config: pop_hash_file | sed -e "s/.* //")
		done
	done
}

testAtCreate(){
	testOK runCl 1 attendee create
	runDbgCl 2 1 attendee create > keypair.1
	runDbgCl 2 1 attendee create > keypair.2
	testFail cmp keypair.1 keypair.2
}

priv=()
pub=()
mkKeypair(){
	local i
	for (( i=1; i<=$1; i++ ))
	do
		runDbgCl 2 1 attendee create > keypair
		priv[i]=$( grep Private keypair | sed -e "s/.* //" )
		pub[i]=$( grep Public keypair | sed -e "s/.* //" )
	done
}

testOrgConfig(){
	mkPopConfig 1 1
	testFail runCl 1 org config pop_desc1.toml
	mkLink 2
	testOK runCl 1 org config pop_desc1.toml
	testFail runCl 2 org config pop_desc1.toml
}

# $1 number of parties $2 number of organizers
mkPopConfig(){
	local n
	for (( n=1; n<=$1; n++ ))
	do
		rm -f pop_desc$n.toml
		cat << EOF > pop_desc$n.toml
Name = "Proof-of-Personhood Party"
DateTime = "2017-08-08 15:00 UTC"
Location = "Earth, City$n"
EOF
	done
	for (( n=1; n<=$2; n++ ))
	do
			cat co$n/public.toml >> pop_desc$n.toml
		if [[ $2 -gt 1 ]]
		then
			local m=$(($n%$2 + 1))
			cat co$m/public.toml >> pop_desc$n.toml
		fi
	done
	rm -f pop_merge.toml
	for (( n=1; n<=$2; n++ ))
	do
		cat << EOF >> pop_merge.toml
[[parties]]
Location = "Earth, City$n"
EOF
		echo "[[parties.servers]]" >> pop_merge.toml
		tail -n +2 co$n/public.toml >> pop_merge.toml
		local m=$(($n%$2 + 1))
		echo "[[parties.servers]]" >> pop_merge.toml
		tail -n +2 co$m/public.toml >> pop_merge.toml
	done
}

testSave(){
	runCoBG 1 2
	mkPopConfig 1 2

	testFail runCl 1 org config pop_desc1.toml
	pkill conode
	sleep .1
	mkLink 2
	pkill conode
	sleep .1
	runCoBG 1 2
	testOK runCl 1 org config pop_desc1.toml
}

mkLink(){
	runCoBG `seq $1`
	for (( serv=1; serv<=$1; serv++ ))
	do
		runCl $serv org link ${addr[$serv]}
		pin=$( grep PIN ${COLOG}$serv.log | sed -e "s/.* //" )
		testOK runCl $serv org link ${addr[$serv]} $pin
	done
}

testOrgLink(){
	runCoBG 1 2
	testOK runCl 1 org link ${addr[1]}
	testGrep PIN cat ${COLOG}1.log
	pin1=$( grep PIN ${COLOG}1.log | sed -e "s/.* //" )
	testFail runCl 1 org link ${addr[1]} abcdefg
	testOK runCl 1 org link ${addr[1]} $pin1
	testOK runCl 2 org link ${addr[2]}
	testGrep PIN cat ${COLOG}2.log
	pin2=$( grep PIN ${COLOG}2.log | sed -e "s/.* //" )
	testOK runCl 2 org link ${addr[2]} $pin2
}

testCheck(){
	runCoBG 1 2 3
	cat co*/public.toml > check.toml
	testOK dbgRun ./$APP -d $DBG_APP check check.toml
}

testBuild(){
	testOK dbgRun ./conode --help
	testOK dbgRun ./$APP --help
}

runCl(){
	local CFG=cl$1
	shift
	dbgRun ./$APP -d $DBG_APP -c $CFG $@
}

runDbgCl(){
	local DBG=$1
	local CFG=cl$2
	shift 2
	DEBUG_COLOR="" ./$APP -d $DBG -c $CFG $@
}

main
