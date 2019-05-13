# This method should be called from the byzcoin/bcadmin/test.sh script

testContractDeferred() {
    run testContractDeferredSpawn
    run testContractDeferredInvoke
}

# We rely on the value contract to make our tests.
testContractDeferredSpawn() {
    # In this test we spawn a value with the --redirect flag and then pipe it
    # to the deferred spawn. We then check the output and see if the proposed
    # transaction is there.
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Add the spawn:value and spawn:deferred rules
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:value" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "spawn:deferred" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract that is piped to the spawn of a deferred
    # contract. We save the output to the OUTRES variable.
    OUTRES=`runBA contract value spawn --value "myValue" --redirect --darc "$ID" --sign "$KEY" | runBA contract deferred spawn --darc "$ID" --sign "$KEY"`

    # Check if we got the expected output
    testGrep "Here is the deferred data:" echo "$OUTRES"
    testGrep "action: spawn:value" echo "$OUTRES"
    testGrep "identities: \[\]" echo "$OUTRES"
    testGrep "counters: \[\]" echo "$OUTRES"
    testGrep "signatures: 0" echo "$OUTRES"
    testGrep "Spawn:	value" echo "$OUTRES"
    testGrep "Args:value" echo "$OUTRES"
    testGrep "Spawned new deferred contract, its instance id is:" echo "$OUTRES"
}

# This method relies on testContractDeferredSpawn() and performs an addProof
# on the proposed transaction and an execProposedTx.
testContractDeferredInvoke() {
    # In this test we do the same as testContractDeferredSpawn() but we then
    # perform an addProof followed by an execProposedTx.
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Add the spawn:value, spawn:deferred, invoke:deferred.addProof and
    # invoke:deferred:execProposedTx rules
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:value" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "spawn:deferred" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "invoke:deferred.addProof" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "invoke:deferred.execProposedTx" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract that is piped to the spawn of a deferred
    # contract.
    OUTRES=`runBA contract value spawn --value "myValue" --redirect --darc "$ID" --sign "$KEY" | runBA contract deferred spawn --darc "$ID" --sign "$KEY"`

    # We know the instance ID is the next line after "Spawned new deferred contract..."
    DEFERRED_INSTANCE_ID=`echo "$OUTRES" | sed -n ' 
        /Spawned new deferred contract/ {
            n
            p
        }'`
    echo -e "Here is the instance ID:\t$DEFERRED_INSTANCE_ID"

    # We know the array conaining the hash to sign is the second line after
    # "- Instruction hashes:" and we remove the "--- " prefix.
    HASH=`echo "$OUTRES" | sed -n ' 
        /- Instruction hashes:/ {
            n
            n
            s/--- //
            p
        }'`
    echo -e "Here is the hash:\t\t$HASH"
    
    testOK runBA contract deferred invoke addProof --instID "$DEFERRED_INSTANCE_ID" --hash "$HASH" --instrIdx 0 --sign "$KEY" --darc "$ID"

    testOK runBA contract deferred invoke execProposedTx --instID "$DEFERRED_INSTANCE_ID" --sign "$KEY"
}