#!/usr/bin/env bash

# Usage: 
#   ./test [options]
# Options:
#   -b   re-builds bcadmin package

DBG_TEST=2
DBG_SRV=2
DBG_APP=1

NBR_SERVERS=4
NBR_SERVERS_GROUP=3

# Clears some env. variables
export -n BC_CONFIG
export -n BC

. "../../libtest.sh"

main(){
    startTest
    buildConode go.dedis.ch/cothority/v3/calypso
    build $APPDIR/../../byzcoin/bcadmin
    run testAuth
    stopTest
}

testAuth(){
    rm -f config/*
    runCoBG 1 2 3
    runBA create public.toml --interval .5s
    bcID=$( ls config/bc-* | sed -e "s/.*bc-\(.*\).cfg/\1/" )

    testFail runCA authorize
    testFail runCA authorize co2/private.toml

    # Create hybrid private with private key from wrong node
    cp co1/private.toml private_wrong.toml
    PRIV2=$( egrep "^Private =" co2/private.toml | sed -e 's/.*"\(.*\)"/\1/' )
    perl -pi -e "s/^Private.*/Private = \"$PRIV2\"/" private_wrong.toml
    testFail runCA authorize private_wrong.toml $bcID

    # Correct signature
    testOK runCA authorize co1/private.toml $bcID

    # Test with signature check disabled
    pkill -9 conode 2> /dev/null
    export COTHORITY_ALLOW_INSECURE_ADMIN=true
    runCoBG 1 2 3
    # Because the old bcID is already stored, create a new one
    # after cleaning the first one
    rm "bc-*.cfg"
    runBA create public.toml --interval .5s
    bcID=$( ls config/bc-* | sed -e "s/.*bc-\(.*\).cfg/\1/" )
    testOK runCA authorize private_wrong.toml $bcID
}

runCA(){
  ./csadmin --debug $DBG_APP "$@"
}

runBA(){
  ./bcadmin -c config/ --debug $DBG_APP "$@"
}

main
