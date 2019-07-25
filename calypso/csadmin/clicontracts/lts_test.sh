# This method should be called from the calypso/csadmin/test.sh script

testContractLTS() {
    run testContractLTSInvoke
}

testContractLTSInvoke() {
    # -bc not given
    testFail runCA contract lts spawn

    rm -f config/*
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    OUTRES=`runCA0 contract lts spawn`
    matchOK "$OUTRES" "^Spawned a new LTS contract. Its instance id is:
[0-9a-f]{64}$" 

    # Create a DARC
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`

    # Fail because it does not have the "spawn:longTermSecret"
    testFail runCA contract lts spawn --darc "$ID"

    # Fail because this identity is not allowed in the default used admin darc
    #
    # TODO: This test should be done, but we end up with a "Got a duplicate
    # transaction, ignoring it" later it we do. 
    #
    # testFail runCA contract lts spawn --sign "$KEY" Let's add the identity and
    # make it pass
    testOK runBA darc rule -rule "spawn:longTermSecret" --identity "$KEY" --replace
    testOK runCA contract lts spawn --sign "$KEY"

    # Let's update the created darc and use it
    testOK runBA darc rule -rule "spawn:longTermSecret" --identity "$KEY" --darc "$ID" --sign "$KEY"
    testOK runCA contract lts spawn --darc "$ID" --sign "$KEY"

    OUTRES=`runCA0 contract lts spawn --darc "$ID" --sign "$KEY"`
    matchOK "$OUTRES" "^Spawned a new LTS contract. Its instance id is:
[0-9a-f]{64}$" 

    # Check the export option
    runCA0 contract lts spawn --darc "$ID" --sign "$KEY" -x > iid.txt
    matchOK "`cat iid.txt`" ^[0-9a-f]{64}$
}