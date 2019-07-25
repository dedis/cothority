# This method should be called from the byzcoin/bcadmin/test.sh script

testContractValue() {
    run testValueSpawn
    run testValueSpawnRedirect
    run testValueInvokeUpdateRedirect
    run testValueGet
    run testValueDel
}

testValueSpawn() {
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

    # Spawn a new value contract, we save the output to the OUTRES variable
    OUTRES=`runBA0 contract value spawn --value "myValue" --darc "$ID" --sign "$KEY"`

    # Check if we got the expected output
    matchOK "$OUTRES" "^Spawned a new value contract. Its instance id is:
[0-9a-f]{64}$"

    # Extract the instance ID of the newly created value instance
    VALUE_INSTANCE_ID=$( echo "$OUTRES" | grep -A 1 "instance id" | sed -n 2p )
    matchOK "$VALUE_INSTANCE_ID" ^[0-9a-f]{64}$

    # Update the value instance based on the instance ID
    testOK runBA contract value invoke update --value "newValue" --instid $VALUE_INSTANCE_ID --darc "$ID" --sign "$KEY"
}

testValueSpawnRedirect() {
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

    # Spawn a new value contract, we save the output to the OUTRES variable
    OUTRES=`runBA0 contract --export value spawn --value "myValue" --darc "$ID" --sign "$KEY"`

    # Check if we got the expected output
    testGrep "value" echo "$OUTRES"
    testGrep "myValue" echo "$OUTRES"
}

testValueInvokeUpdateRedirect() {
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

    # Spawn a new value contract, we save the output to the OUTRES variable
    OUTRES=`runBA0 contract --export value invoke update --value "newValue" -i aef123 --darc "$ID" --sign "$KEY"`

    # Check if we got the expected output
    testGrep "value" echo "$OUTRES"
    testGrep "newValue" echo "$OUTRES"
}

testValueDeleteRedirect() {
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

    # Spawn a new value contract, we save the output to the OUTRES variable
    OUTRES=`runBA0 contract --export value invoke update --value "newValue" -i aef123 --darc "$ID" --sign "$KEY"`

    # Check if we got the expected output
    testGrep "value" echo "$OUTRES"
    testGrep "newValue" echo "$OUTRES"
}

testValueGet() {
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

    # Spawn a new value contract, we save the output to the OUTRES variable
    OUTRES=`runBA0 contract value spawn --value "myValue" --darc "$ID" --sign "$KEY"`

    # Check if we got the expected output
    matchOK "$OUTRES" "^Spawned a new value contract. Its instance id is:
[0-9a-f]{64}$"

    # Extract the instance ID of the newly created value instance
    VALUE_INSTANCE_ID=$( echo "$OUTRES" | grep -A 1 "instance id" | sed -n 2p )
    matchOK "$VALUE_INSTANCE_ID" ^[0-9a-f]{64}$

    # Use the "get" function and save the output to the OUTPUT variable
    OUTRES=`runBA0 contract value get --instid "$VALUE_INSTANCE_ID"`

    testGrep "myValue" echo "$OUTRES"

    # Update the value instance based on the instance ID
    testOK runBA contract value invoke update --value "newValue" --instid $VALUE_INSTANCE_ID --darc "$ID" --sign "$KEY"

    # Use the "get" function and save the output to the OUTPUT variable
    OUTRES=`runBA0 contract value get --instid "$VALUE_INSTANCE_ID"`

    testGrep "newValue" echo "$OUTRES"

    # Try to get a wrong instance ID
    testFail runBA contract value get --instid deadbeef
}

testValueDel() {
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

    # Spawn a new value contract, we save the output to the OUTRES variable
    OUTRES=`runBA0 contract value spawn --value "myValue" --darc "$ID" --sign "$KEY"`

    # Check if we got the expected output
    matchOK "$OUTRES" "^Spawned a new value contract. Its instance id is:
[0-9a-f]{64}$"

    # Extract the instance ID of the newly created value instance
    VALUE_INSTANCE_ID=$( echo "$OUTRES" | grep -A 1 "instance id" | sed -n 2p )
    matchOK "$VALUE_INSTANCE_ID" ^[0-9a-f]{64}$

    # Use the "get" function to retrieve the contract. It should pass.
    testOK runBA contract value get --instid "$VALUE_INSTANCE_ID"

    # Use the "delete" function
    testOK runBA contract value delete --instid "$VALUE_INSTANCE_ID" --darc "$ID" --sign "$KEY"

    # Use the "get" function to retrieve the contract. It should fail.
    testFail runBA contract value get --instid "$VALUE_INSTANCE_ID"

    # Use the "delete" function, should fail since it does not exist anymore
    testFail runBA contract value delete --instid "$VALUE_INSTANCE_ID" --darc "$ID" --sign "$KEY"
}