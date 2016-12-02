#!/usr/bin/env bash

DBG_SRV=${DBG_SRV:-0}

runCoCfg(){
	rm -rf co$1
	mkdir co$1
    echo -e "127.0.0.1:200$1\nco$1\n\n" | dbgRun runCo $1 setup
}

runCoBG(){
    nb=$1
    shift
    testOut "starting cothority-server #$nb"
#    ( ./cothorityd -d $DBG_SRV -c co$nb/config.toml $@ 2>&1 > /dev/null & )
    ( ./cothorityd -d $DBG_SRV -c co$nb/config.toml $@ & )
}

runCo(){
    nb=$1
    shift
    testOut "starting cothority-server #$nb"
    dbgRun ./cothorityd -d $DBG_SRV -c co$nb/config.toml $@
}

# first argument: number of configurations to create
# second argument: number of cothorities to run
# For each cothority that is started, its definition is copied to
# the group.toml-file.
cothoritySetup(){
	nbrCfg=${1:-3}
	nbrRun=${2:-2}
    DBG_OLD=$DBG_SHOW
    DBG_SHOW=0
    for n in $( seq $nbrCfg ); do
    	runCoCfg $n
    done
    rm group.toml
    for n in $( seq $nbrRun ); do
    	runCoBG $n
    	if [ $n != $nbrCfg ]; then
    		tail -n 4 co$n/group.toml >> group.toml
    	fi
    done
    sleep 1
    DBG_SHOW=$DBG_OLD
}
