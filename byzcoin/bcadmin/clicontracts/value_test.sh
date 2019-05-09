# This method should be called from the byzcoin/bcadmin/test.sh script

testContractValue() {
    run testSpawn
    run testSpawnRedirect
}

testSpawn() {
   # In this test we spawn a value contract and then update its value
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Add the spawn:value and invoke:value.update rules
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:value" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "invoke:value.update" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract, we save the output to the res.txt file
    OUTFILE=res.txt && testOK runBA contract value spawn --value "myValue" --darc "$ID" --sign "$KEY"
    OUTFILE=""

    # Check if we got the expected output
    testGrep "Spawned new value contract. Instance id is:" cat res.txt

    # Extract the instance ID of the newly created value instance
    VALUE_INSTANCE_ID=`sed -n 2p res.txt`

    # Update the value instance based on the instance ID
    testOK runBA contract value invoke update --value "newValue" --instID $VALUE_INSTANCE_ID --darc "$ID" --sign "$KEY"
}

testSpawnRedirect() {
   # In this test we spawn a value with the --redirect flag
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Add the spawn:value and invoke:value.update rules
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:value" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "invoke:value.update" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract, we save the output to the res.txt file
    OUTFILE=res.txt && testOK runBA contract value spawn --value "myValue" --redirect --darc "$ID" --sign "$KEY"
    OUTFILE=""

    # Check if we got the expected output
    testGrep "myValue" cat res.txt
}