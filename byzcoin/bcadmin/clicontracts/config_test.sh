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
    OUTRES=`runBA0 contract config invoke updateConfig`

    matchOK "$OUTRES" "^Config contract updated! \(instance ID is [a-f0-9]{64}\)
Here is the config data:
- ChainConfig:
-- BlockInterval: [a-z0-9]+
-- Roster: \{.*\}
-- MaxBlockSize: [0-9]+
-- DarcContractIDs:
--- darc contract ID 0: darc$"

    # Update all the arguments. We check if the return corresponds.
    OUTRES=`runBA0 contract config invoke updateConfig\
                --blockInterval 7s\
                --maxBlockSize 5000000\
                --darcContractIDs darc,darc2,darc3`
    
    matchOK "$OUTRES" "^Config contract updated! \(instance ID is [a-f0-9]{64}\)
Here is the config data:
- ChainConfig:
-- BlockInterval: 7s
-- Roster: \{.*\}
-- MaxBlockSize: 5000000
-- DarcContractIDs:
--- darc contract ID 0: darc
--- darc contract ID 1: darc2
--- darc contract ID 2: darc3$"

}

# In this test we simply get the config contract and check the result.
testContractConfigGet() {
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Get the config instance
    OUTRES=`runBA0 contract config get`

    matchOK "$OUTRES" "^Here is the config data:
- ChainConfig:
-- BlockInterval: [a-z0-9]+
-- Roster: \{.*\}
-- MaxBlockSize: [0-9]+
-- DarcContractIDs:
--- darc contract ID 0: darc$"
}