# This method should be called from the byzcoin/bcadmin/test.sh script

testContractConfig() {
    run testContractConfigInvoke
    run testContractConfigGet
}

# In this test we check the behavior of the invoke:update_config function. We
# first perform an update with nothing, then we update all the parameter and
# check the result.
testContractConfigInvoke() {
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Run an update with no argument, we should have in return the current
    # config.
    OUTRES=`runBA contract config invoke updateConfig`

    testGrep "Config contract updated! \(instance ID is [a-f0-9]+\)" echo "$OUTRES"
    testGrep "Here is the config data:" echo "$OUTRES"
    testGrep "ChainConfig" echo "$OUTRES"
    testGrep "\- BlockInterval: [a-z0-9]+" echo "$OUTRES"
    testGrep "\- Roster: \{.*\}" echo "$OUTRES"
    testGrep "\- MaxBlockSize: [0-9]+" echo "$OUTRES"
    testGrep "\- DarcContractIDs:" echo "$OUTRES"
    testGrep "\-\- darc contract ID 0: darc" echo "$OUTRES"

    # Update all the arguments. We check if the return corresponds.
    OUTRES=`runBA contract config invoke updateConfig\
                --blockInterval 7s\
                --maxBlockSize 5000000\
                --darcContractIDs darc,darc2,darc3`
    
    testGrep "Config contract updated! \(instance ID is [a-f0-9]+\)" echo "$OUTRES"
    testGrep "Here is the config data:" echo "$OUTRES"
    testGrep "ChainConfig" echo "$OUTRES"
    testGrep "\- BlockInterval: 7s" echo "$OUTRES"
    testGrep "\- Roster: \{.*\}" echo "$OUTRES"
    testGrep "\- MaxBlockSize: 5000000" echo "$OUTRES"
    testGrep "\- DarcContractIDs:" echo "$OUTRES"
    testGrep "\-\- darc contract ID 0: darc" echo "$OUTRES"
    testGrep "\-\- darc contract ID 1: darc2" echo "$OUTRES"
    testGrep "\-\- darc contract ID 2: darc3" echo "$OUTRES"
}

# In this test we simply get the config contract and check the result.
testContractConfigGet() {
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Get the config instance
    OUTRES=`runBA contract config get`

    # Check the result
    testGrep "Here is the config data:" echo "$OUTRES"
    testGrep "ChainConfig" echo "$OUTRES"
    testGrep "\- BlockInterval: [a-z0-9]+" echo "$OUTRES"
    testGrep "\- Roster: \{.*\}" echo "$OUTRES"
    testGrep "\- MaxBlockSize: [0-9]+" echo "$OUTRES"
    testGrep "\- DarcContractIDs:" echo "$OUTRES"
    testGrep "\-\- darc contract ID 0: darc" echo "$OUTRES"
}