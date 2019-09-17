# This method should be called from the byzcoin/bcadmin/test.sh script

testContractName() {
    run testNameSpawn
    run tesNameInvokeAdd
    run testNameInvokeRemove
    run testNameGet
}

testNameSpawn() {
   # In this test we spawn the name contract a first time, then we try to do it
   # a second time. Since the name contract is a singleton, the second time
   # should fail.
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    testOK runBA0 contract name spawn
    testFail runBA0 contract name spawn
}

# Rely on:
# - bcadmin contract name spawn
# - bcadmin contract value spawn
tesNameInvokeAdd() {
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    OUTRES=`runBA0 contract name spawn`
    matchOK "$OUTRES" "^Spawned a new namne contract. Its instance id is:
[0-9a-f]{64}"    

    # Add the spawn:value rule
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:value" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn a new value contract, we save the output to the OUTRES variable
    OUTRES=`runBA0 contract value spawn --value "Hello world" --darc "$ID" --sign "$KEY"`

    # Check if we got the expected output
    matchOK "$OUTRES" "^Spawned a new value contract. Its instance id is:
[0-9a-f]{64}$"

    # Extract the instance ID of the newly created value instance
    VALUE_INSTANCE_ID=$( echo "$OUTRES" | grep -A 1 "instance id" | sed -n 2p )
    matchOK "$VALUE_INSTANCE_ID" ^[0-9a-f]{64}$

    # This should not work since the action "_name:value" is not added in the
    # darc
    testFail runBA0 contract name invoke add -i $VALUE_INSTANCE_ID -name "myValue" --sign "$KEY"

    # Now let's add the rule in the darc
    testOK runBA0 darc rule -rule "_name:value" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Now let's add a name resolver
    OUTRES=`runBA0 contract name invoke add -i $VALUE_INSTANCE_ID -name "myValue" --sign "$KEY"`
    matchOK "$OUTRES" "^Added a new naming instance with name 'myValue'. Its instance id is:
[0-9a-f]{64}$"

    # Extract the instance ID of the newly created name instance
    NAME_INSTANCE_ID=$( echo "$OUTRES" | grep -A 1 "instance id" | sed -n 2p )
    matchOK "$NAME_INSTANCE_ID" ^[0-9a-f]{64}$

    # Name must be uniq, if I try to add the same name it should fail
    testFail runBA0 contract name invoke add -i $VALUE_INSTANCE_ID -name "myValue" --sign "$KEY"

    # Let's use the "--append" option that appends a random string to the name
    OUTRES=`runBA0 contract name invoke add -i $VALUE_INSTANCE_ID -name "myValue" --sign "$KEY" -a`
    matchOK "$OUTRES" "^Added a new naming instance with name 'myValue-[a-zA-Z]{16}'. Its instance id is:
[0-9a-f]{64}$"

    # Let's add multiple instances
    
    # We create a second value instance
    OUTRES=`runBA0 contract value spawn --value "Hi there" --darc "$ID" --sign "$KEY"`
    VALUE_INSTANCE_ID2=$( echo "$OUTRES" | grep -A 1 "instance id" | sed -n 2p )
    matchOK "$VALUE_INSTANCE_ID2" ^[0-9a-f]{64}$

    OUTRES=`runBA0 contract name invoke add -i $VALUE_INSTANCE_ID -i $VALUE_INSTANCE_ID2 -name "myName" --sign "$KEY"`
    matchOK "$OUTRES" "^Added a new naming instance with name 'myName-[a-zA-Z]{16}'. Its instance id is:
[0-9a-f]{64}
Added a new naming instance with name 'myName-[a-zA-Z]{16}'. Its instance id is:
[0-9a-f]{64}$"
}

# Rely on:
# - bcadmin contract name spawn
# - bcadmin contract value spawn
testNameInvokeRemove() {
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    testOK runBA contract name spawn

    # Create a darc and add the rules
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "_name:value" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runBA darc rule -rule "spawn:value" --identity "$KEY" --darc "$ID" --sign "$KEY"

    OUTRES=`runBA0 contract value spawn --value "Hello world" --darc "$ID" --sign "$KEY"`
    VALUE_INSTANCE_ID=$( echo "$OUTRES" | grep -A 1 "instance id" | sed -n 2p )
    matchOK "$VALUE_INSTANCE_ID" ^[0-9a-f]{64}$

    # Remove before creation, it should fail
    testFail runBA contract name invoke remove --name "myKey" --sign "$KEY" -i "$VALUE_INSTANCE_ID"
    # Add name resolver
    testOK runBA contract name invoke add --name "myKey" --sign "$KEY" -i "$VALUE_INSTANCE_ID"
    # Remove name resolver
    testOK runBA contract name invoke remove --name "myKey" --sign "$KEY" -i "$VALUE_INSTANCE_ID"
    # Remove a second time, it should fail
    testFail runBA contract name invoke remove --name "myKey" --sign "$KEY" -i "$VALUE_INSTANCE_ID"
}

# Rely on:
# - bcadmin contract name spawn 
# - bcadmin contract value spawn
testNameGet() {
    # In this test we
   runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Should fail if the name contract is not spawned
    testFail runBA0 contract name get

    # Now we spawn the name contract...
    testOK runBA contract name spawn

    # ... and we can get it.
    OUTRES=`runBA0 contract name get`
    matchOK "$OUTRES" "Here is the naming data:
- ContractNamingBody:
-- Latest: 0000000000000000000000000000000000000000000000000000000000000000"
}