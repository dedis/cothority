#!/usr/bin/env bash

# Usage:
#   ./test [options]
# Options:
#   -b   re-builds bcadmin package

# shellcheck disable=SC2034  # Used in libtest.sh
DBG_TEST=1
# shellcheck disable=SC2034  # Used in libtest.sh
DBG_SRV=2
DBG_APP=2

# shellcheck disable=SC2034  # Used in libtest.sh
NBR_SERVERS=4
# shellcheck disable=SC2034  # Used in libtest.sh
NBR_SERVERS_GROUP=3

# Clears some env. variables
export -n BC_CONFIG
export -n BC
export -n BEVM_ID
export BC_WAIT=true

. "../../libtest.sh"

main(){
    startTest
    buildConode go.dedis.ch/cothority/v3/bevm
    build "${APPDIR}/../bevmadmin"
    build "${APPDIR}/../../byzcoin/bcadmin"
    [[ ! -x ./bevmadmin ]] && exit 1
    [[ ! -x ./bevmclient ]] && exit 1

    run testBevmInteraction
    stopTest
}

initBevm(){
    # Create BEvm admin identity
    runBcAdmin key --save ./bevm_admin_key.txt
    BEVM_ADMIN=$( cat ./bevm_admin_key.txt )

    # Create BEvm user identity
    runBcAdmin key --save ./bevm_user_key.txt
    BEVM_USER=$( cat ./bevm_user_key.txt )

    # Create BEvm Darc
    runBcAdmin darc add \
        --unrestricted \
        --desc "BEvm Darc" \
        --identity "${BEVM_ADMIN}" \
        --out_id ./bevm_darc_id.txt
    BEVM_DARC=$( cat ./bevm_darc_id.txt )

    # Initialize BEvm Darc
    # 'spawn' and 'delete' granted to BEVM_ADMIN
    runBcAdmin darc rule \
        --sign "${BEVM_ADMIN}" \
        --darc "${BEVM_DARC}" \
        --rule "spawn:bevm" \
        --identity "${BEVM_ADMIN}"
    runBcAdmin darc rule \
        --sign "${BEVM_ADMIN}" \
        --darc "${BEVM_DARC}" \
        --rule "delete:bevm" \
        --identity "${BEVM_ADMIN}"
    # 'credit' and 'transaction' granted to BEVM_USER
    runBcAdmin darc rule \
        --sign "${BEVM_ADMIN}" \
        --darc "${BEVM_DARC}" \
        --rule "invoke:bevm.credit" \
        --identity "${BEVM_USER}"
    runBcAdmin darc rule \
        --sign "${BEVM_ADMIN}" \
        --darc "${BEVM_DARC}" \
        --rule "invoke:bevm.transaction" \
        --identity "${BEVM_USER}"
    # Check
    runBcAdmin darc show \
        --darc "${BEVM_DARC}"
}

testBevmInteraction(){
    rm -f config/*
    runCoBG 1 2 3

    # Initialize ByzCoin
    runBcAdmin create public.toml --interval .5s
    export BC=$( ls config/bc-* )

    initBevm

    # Create BEvm instance
    testOK runBevmAdmin spawn \
        --sign "${BEVM_ADMIN}" \
        --darc "${BEVM_DARC}" \
        --outID ./bevm_instance_id.txt
    export BEVM_ID=$( cat ./bevm_instance_id.txt )

    # Create BEvm account
    testOK runBevmClient createAccount

    # Credit account
    # Cannot credit as BEVM_ADMIN
    testFail runBevmClient creditAccount \
        --sign "${BEVM_ADMIN}" \
        10
    testOK runBevmClient creditAccount \
        --sign "${BEVM_USER}" \
        10

    # Check account balance
    testGrep "10 Ether, 0 Wei" runBevmClient getAccountBalance \
        --sign "${BEVM_USER}"

    # Deploy Candy contract
    testOK runBevmClient deployContract \
        --sign "${BEVM_USER}" \
        "${APPDIR}/../testdata/Candy/Candy_sol_Candy.abi" \
        "${APPDIR}/../testdata/Candy/Candy_sol_Candy.bin" \
        100

    # Check Candy balance
    testGrep " 100 " runBevmClient call \
        --sign "${BEVM_USER}" \
        getRemainingCandies

    # Eat some candy
    testOK runBevmClient transaction \
        --sign "${BEVM_USER}" \
        eatCandy \
        58

    # Check Candy balance
    testGrep " 42 " runBevmClient call \
        --sign "${BEVM_USER}" \
        getRemainingCandies

    # Delete BEvm instance
    # Cannot delete as BEVM_USER
    testFail runBevmAdmin delete \
        --sign "${BEVM_USER}" \
        --bevmID "${BEVM_ID}"
    testOK runBevmAdmin delete \
        --sign "${BEVM_ADMIN}" \
        --bevmID "${BEVM_ID}"
}

runBcAdmin(){
    ./bcadmin --config config/ --debug "${DBG_APP}" "$@"
}

runBevmAdmin(){
    ./bevmadmin --config config/ --debug "${DBG_APP}" "$@"
}

runBevmClient(){
    ./bevmclient --config config/ --debug "${DBG_APP}" "$@"
}

main
