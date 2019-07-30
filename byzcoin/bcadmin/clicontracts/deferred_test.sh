# This method should be called from the byzcoin/bcadmin/test.sh script

testContractDeferred() {
    run testDeferredSpawn
    run testDeferredInvoke
    run testDeferredGet
    run testDeferredDel
    run testDeferredInvokeDeferred
}

# We rely on the value contract to make our tests.
testDeferredSpawn() {
    # In this test we spawn a value with the --export (-x) flag and then pipe it
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
    OUTRES=`runBA0 contract -x value spawn --value "myValue" --darc "$ID" --sign "$KEY" | runBA0 contract deferred spawn --darc "$ID" --sign "$KEY"`

    matchOK "$OUTRES" "^Spawned a new deferred contract. Its instance id is:
[0-9a-f]{64}
Here is the deferred data:
- Proposed Tx:
-- Instruction 0:
--- instruction:
---- hash: [0-9a-f]{64}
---- instID: [0-9a-f]{64}
---- action: spawn:value
---- identities: \[\]
---- counters: \[\]
---- signatures: 0
---- Spawn:
----- ContractID: value
----- Args:
------ value:
------- \"myValue\"
- Expire Block Index: [0-9]+
- Instruction hashes:
-- hash 0:
--- [0-9a-f]{64}
- Max num execution: 1
- Exec results: $"

}

# This method relies on testDeferredSpawn() and performs an addProof
# on the proposed transaction and an execProposedTx.
testDeferredInvoke() {
    # In this test we do the same as testDeferredSpawn() but we then
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
    OUTRES=`runBA0 contract -x value spawn --value "myValue" --darc "$ID" --sign "$KEY" | runBA0 contract deferred spawn --darc "$ID" --sign "$KEY"`

    # We know the instance ID is the next line after "Spawned new deferred contract..."
    DEFERRED_INSTANCE_ID=`echo "$OUTRES" | sed -n ' 
        /Spawned a new deferred contract/ {
            n
            p
        }'`
    matchOK "$DEFERRED_INSTANCE_ID" ^[0-9a-f]{64}$

    # We know the array conaining the hash to sign is the second line after
    # "- Instruction hashes:" and we remove the "--- " prefix.
    HASH=`echo "$OUTRES" | sed -n ' 
        /- Instruction hashes:/ {
            n
            n
            s/--- //
            p
        }'`
    matchOK "$HASH" ^[0-9a-f]{64}$
    
    testOK runBA contract deferred invoke addProof --instid "$DEFERRED_INSTANCE_ID" --hash "$HASH" --instrIdx 0 --sign "$KEY" --darc "$ID"

    testOK runBA contract deferred invoke execProposedTx --instid "$DEFERRED_INSTANCE_ID" --sign "$KEY"
}

testDeferredGet() {
    # In this test we spawn a deferred contract and then retrieve the value
    # stored with the "get" function. We then perform an addProof and test if we
    # can get the updated value, ie. the identity added. We partially use the
    # same code as the spawn and update function.
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Add the necessary rules
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:value" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "spawn:deferred" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "invoke:deferred.addProof" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract that is piped to the spawn of a deferred
    # contract.
    OUTRES=`runBA0 contract -x value spawn --value "myValue" --darc "$ID" --sign "$KEY" | runBA0 contract deferred spawn --darc "$ID" --sign "$KEY"`

    # We know the instance ID is the next line after "Spawned new deferred contract..."
    DEFERRED_INSTANCE_ID=`echo "$OUTRES" | sed -n ' 
        /Spawned a new deferred contract/ {
            n
            p
        }'`
    matchOK "$DEFERRED_INSTANCE_ID" ^[0-9a-f]{64}$

    # We know the array containing the hash to sign is the second line after
    # "- Instruction hashes:" and we remove the "--- " prefix.
    HASH=`echo "$OUTRES" | sed -n ' 
        /- Instruction hashes:/ {
            n
            n
            s/--- //
            p
        }'`
    matchOK "$HASH" ^[0-9a-f]{64}$

    # We now use the get function to check if we have the right informations:
    OUTRES=`runBA0 contract deferred get --instid $DEFERRED_INSTANCE_ID`

    matchOK "$OUTRES" "^- Proposed Tx:
-- Instruction 0:
--- instruction:
---- hash: [0-9a-f]{64}
---- instID: [0-9a-f]{64}
---- action: spawn:value
---- identities: \[\]
---- counters: \[\]
---- signatures: 0
---- Spawn:
----- ContractID: value
----- Args:
------ value:
------- \"myValue\"
- Expire Block Index: [0-9]+
- Instruction hashes:
-- hash 0:
--- [0-9a-f]{64}
- Max num execution: 1
- Exec results: $"
    
    testOK runBA contract deferred invoke addProof --instid "$DEFERRED_INSTANCE_ID" --hash "$HASH" --instrIdx 0 --sign "$KEY" --darc "$ID"

    # Since we performed an addProof, the result should now contrain a new
    # identity and the field signature set to 1.
    OUTRES=`runBA0 contract deferred get --instid $DEFERRED_INSTANCE_ID`

    matchOK "$OUTRES" "^- Proposed Tx:
-- Instruction 0:
--- instruction:
---- hash: [0-9a-f]{64}
---- instID: [0-9a-f]{64}
---- action: spawn:value
---- identities: \[$KEY\]
---- counters: \[\]
---- signatures: 1
---- Spawn:
----- ContractID: value
----- Args:
------ value:
------- \"myValue\"
- Expire Block Index: [0-9]+
- Instruction hashes:
-- hash 0:
--- [0-9a-f]{64}
- Max num execution: 1
- Exec results: $"

    # Try to get a wrong instance ID
    testFail runBA contract deferred get --instid deadbeef
}

testDeferredDel() {
    # In this test we spawn a deferred contract, delete it and check if we can
    # get it. Uses partially the code of the spawn test.
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Add the necessary rules
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:value" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "spawn:deferred" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "delete:deferred" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract that is piped to the spawn of a deferred
    # contract.
    OUTRES=`runBA0 contract -x value spawn --value "myValue" --darc "$ID" --sign "$KEY" | runBA0 contract deferred spawn --darc "$ID" --sign "$KEY"`

    # We know the instance ID is the next line after "Spawned new deferred contract..."
    DEFERRED_INSTANCE_ID=`echo "$OUTRES" | sed -n ' 
        /Spawned a new deferred contract/ {
            n
            p
        }'`
    matchOK "$DEFERRED_INSTANCE_ID" ^[0-9a-f]{64}$

    # We should be able to get the created deferred instance
    testOK runBA contract deferred get --instid $DEFERRED_INSTANCE_ID
    
    # We delete the instance
    testOK runBA contract deferred delete --instid $DEFERRED_INSTANCE_ID --darc "$ID" --sign "$KEY"
    
    # Now we shouldn't be able to get it back
    testFail runBA contract deferred get --instid $DEFERRED_INSTANCE_ID

    # Use the "delete" function, should fail since it does not exist anymore
    testFail runBA contract deferred delete --instid "$VALUE_INSTANCE_ID" --darc "$ID" --sign "$KEY" --instid $DEFERRED_INSTANCE_ID
}

# This method relies on testDeferredSpawn() and performs an addProof
# on the proposed transaction and an execProposedTx.
testDeferredInvokeDeferred() {
    # In this test we normally create a deferred spawn:value but then we
    # invoke a deferred deferred:invoke.addProof. So the addProof operation
    # will be made with a deferred contract. Crazy hu?
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
    OUTRES=`runBA0 contract -x value spawn --value "myValue" --darc "$ID" --sign "$KEY" | runBA0 contract deferred spawn --darc "$ID" --sign "$KEY"`

    # We know the instance ID is the next line after "Spawned new deferred contract..."
    DEFERRED_INSTANCE_ID=`echo "$OUTRES" | sed -n ' 
        /Spawned a new deferred contract/ {
            n
            p
        }'`
    matchOK "$DEFERRED_INSTANCE_ID" ^[0-9a-f]{64}$

    # We know the array conaining the hash to sign is the second line after
    # "- Instruction hashes:" and we remove the "--- " prefix.
    HASH=`echo "$OUTRES" | sed -n ' 
        /- Instruction hashes:/ {
            n
            n
            s/--- //
            p
        }'`
    matchOK "$HASH" ^[0-9a-f]{64}$
    
    # Now we create a new deferred contract that performs an addProof on the
    # first deferred contract
    OUTRES2=`runBA0 contract -x deferred invoke addProof --instid "$DEFERRED_INSTANCE_ID" --hash "$HASH"\
                                                   --instrIdx 0 --sign "$KEY" --darc "$ID" |\
                                                   runBA0 contract deferred spawn --darc "$ID" --sign "$KEY"`

    # We know the instance ID is the next line after "Spawned new deferred contract..."
    DEFERRED_INSTANCE_ID_2=`echo "$OUTRES2" | sed -n ' 
        /Spawned a new deferred contract/ {
            n
            p
        }'`
    echo -e "Here is the instance ID:\t$DEFERRED_INSTANCE_ID_2"

    # We know the array conaining the hash to sign is the second line after
    # "- Instruction hashes:" and we remove the "--- " prefix.
    HASH2=`echo "$OUTRES2" | sed -n ' 
        /- Instruction hashes:/ {
            n
            n
            s/--- //
            p
        }'`
    matchOK "$HASH2" ^[0-9a-f]{64}$

    # Now we must execute the second deferred contract that will add a proof to
    # the first one.
    testOK runBA contract deferred invoke addProof --instid "$DEFERRED_INSTANCE_ID_2" --hash "$HASH2"\
                                                   --instrIdx 0 --sign "$KEY" --darc "$ID"
    testOK runBA contract deferred invoke execProposedTx --instid "$DEFERRED_INSTANCE_ID_2" --sign "$KEY" --darc "$ID"
    
    runBA contract deferred get --instid "$DEFERRED_INSTANCE_ID"
    testOK runBA contract deferred invoke execProposedTx --instid "$DEFERRED_INSTANCE_ID" --sign "$KEY" --darc "$ID"
}