# This method should be called from the byzcoin/bcadmin/test.sh script

testContractValue() {
    run testSpawn
    run testSpawnRedirect
    run testInvokeUpdateRedirect
    run testGet
    run testDel
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
    testOK runBA contract value invoke update --value "newValue" --instid $VALUE_INSTANCE_ID --darc "$ID" --sign "$KEY"
}

testSpawnRedirect() {
   # In this test we spawn a value with the --export flag
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Add the rules
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:value" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract, we save the output to the res.txt file
    OUTFILE=res.txt && testOK runBA contract --export value spawn --value "myValue" --darc "$ID" --sign "$KEY"
    OUTFILE=""

    # Check if we got the expected output
    testGrep "value" cat res.txt
    testGrep "myValue" cat res.txt
}

testInvokeUpdateRedirect() {
   # In this test we update a fake instance with exported output
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Add the rules
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "invoke:value.update" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract, we save the output to the res.txt file
    OUTFILE=res.txt && testOK runBA contract --export value invoke update --value "newValue" -i aef123 --darc "$ID" --sign "$KEY"
    OUTFILE=""

    # Check if we got the expected output
    testGrep "value" cat res.txt
    testGrep "newValue" cat res.txt
}

testDeleteRedirect() {
   # In this test we delete a fake instance with exported output
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Add the rules
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "delete:value" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract, we save the output to the res.txt file
    OUTFILE=res.txt && testOK runBA contract --export value invoke update --value "newValue" -i aef123 --darc "$ID" --sign "$KEY"
    OUTFILE=""

    # Check if we got the expected output
    testGrep "value" cat res.txt
    testGrep "newValue" cat res.txt
}

testGet() {
    # In this test we spawn a value contract and then retrieve the value stored
    # with the "get" function. We then perform an update and test if we can get
    # the updated value. We partially use the same code as the spawn function.
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Add the delete rule
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:value" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "invoke:value.update" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "delete:value" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract, we save the output to the res.txt file
    OUTFILE=res.txt && testOK runBA contract value spawn --value "myValue" --darc "$ID" --sign "$KEY"
    OUTFILE=""

    # Check if we got the expected output
    testGrep "Spawned new value contract. Instance id is:" cat res.txt

    # Extract the instance ID of the newly created value instance
    VALUE_INSTANCE_ID=`sed -n 2p res.txt`

    # Use the "get" function and save the output to the res.txt file. This file
    # should contain the saved value.
    OUTFILE=res.txt && testOK runBA contract value get --instid "$VALUE_INSTANCE_ID"
    OUTFILE=""

    testGrep "myValue" cat res.txt

    # Update the value instance based on the instance ID
    testOK runBA contract value invoke update --value "newValue" --instid $VALUE_INSTANCE_ID --darc "$ID" --sign "$KEY"

    # Use the "get" function and save the output to the res.txt file. This file
    # should contain the newly updated value.
    OUTFILE=res.txt && testOK runBA contract value get --instid "$VALUE_INSTANCE_ID"
    OUTFILE=""

    testGrep "newValue" cat res.txt

    # Try to get a wrong instance ID
    testFail runBA contract value get --instid deadbeef
}

testDel() {
    # In this test we spawn a value contract, delete it and check if we can get
    # it. Uses partially the code of the spawn test.
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Add the rules
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:value" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "delete:value" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract, we save the output to the res.txt file
    OUTFILE=res.txt && testOK runBA contract value spawn --value "myValue" --darc "$ID" --sign "$KEY"
    OUTFILE=""

    # Check if we got the expected output
    testGrep "Spawned new value contract. Instance id is:" cat res.txt

    # Extract the instance ID of the newly created value instance
    VALUE_INSTANCE_ID=`sed -n 2p res.txt`

    # Use the "get" function to retrieve the contract. It should pass.
    testOK runBA contract value get --instid "$VALUE_INSTANCE_ID"

    # Use the "delete" function
    testOK runBA contract value delete --instid "$VALUE_INSTANCE_ID" --darc "$ID" --sign "$KEY"

    # Use the "get" function to retrieve the contract. It should fail.
    testFail runBA contract value get --instid "$VALUE_INSTANCE_ID"

    # Use the "delete" function, should fail since it does not exist anymore
    testFail runBA contract value delete --instid "$VALUE_INSTANCE_ID" --darc "$ID" --sign "$KEY"
}