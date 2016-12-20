#!/usr/bin/env bash

if [ ! "$LIBTEST" ]; then
  . $GOPATH/src/github.com/dedis/onet/app/libtest.sh
fi

DBG_SRV=${DBG_SRV:-0}
NBR_SERVERS=${NBR_SERVERS:-3}
NBR_SERVERS_GROUP=${NBR_SERVERS_GROUP:-2}
COLOG=/tmp/cothority_

cothorityBuild(){
	local service=$1
    for n in $(seq $NBR_SERVERS); do
        co=co$n
        rm -f $co/*
        mkdir -p $co
    done
    local pkg=$( realpath $BUILDDIR | sed -e "s:$GOPATH/src/::" )
    local cot=$GOPATH/src/github.com/dedis/cothority/cothority.go
    local cotdir=$( mktemp -d )/cothority
    mkdir -p $cotdir
    sed -e "s:.*TEST_LINE.*:_ \"$pkg/$service\":" $cot > $cotdir/cothority.go
	build $cotdir
	rm -rf $cotdir
	cothoritySetup
}

cothoritySetup(){
    DBG_OLD=$DBG_SHOW
    DBG_SHOW=0
	rm -f group.toml
    for n in $( seq $NBR_SERVERS ); do
    	runCoCfg $n
    	if [ $n -le $NBR_SERVERS_GROUP ]; then
		    tail -n 4 co$n/group.toml >> group.toml
		fi
	done
    DBG_SHOW=$DBG_OLD
}

testCothority(){
    runCoBG 1 2
    sleep 1
    cp co1/group.toml testgroup.toml
    tail -n 4 co2/group.toml >> testgroup.toml
    testOK runCo 1 check -g testgroup.toml
    tail -n 4 co3/group.toml >> testgroup.toml
    testFail runCo 1 check -g testgroup.toml
}

runCoCfg(){
	mkdir -p co$1
    echo -e "127.0.0.1:200$(( 2 * $1 ))\nNew Cothority $1\nco$1\n" | dbgRun runCo $1 setup
}

runCoBG(){
    local
    for nb in $@; do
    	testOut "starting cothority-server #$nb"
    	( ./cothority -d $DBG_SRV -c co$nb/config.toml $@ | tee $COLOG$nb.log & )
    done
}

runCo(){
    local nb=$1
    shift
    testOut "starting cothority-server #$nb"
    dbgRun ./cothority -d $DBG_SRV -c co$nb/config.toml $@
}
