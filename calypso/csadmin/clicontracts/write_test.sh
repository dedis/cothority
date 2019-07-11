# This method should be called from the calypso/csadmin/test.sh script

testContractWrite() {
    run testContractWriteInvoke
}

# rely on:
# - csadmin contract lts spawn
# - csadmin authorize
testContractWriteInvoke() {
    rm -f config/*
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Create a DARC
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:longTermSecret" --identity "$KEY" --darc "$ID" --sign "$KEY"

    # Spawn LTS
    OUTRES=`runCA contract lts spawn --darc "$ID" --sign "$KEY"`
    LTS_ID=`echo "$OUTRES" | sed -n '2p'`
    matchOK $LTS_ID ^[0-9a-f]{64}$
    # Authorize nodes
    bcID=$( ls config/bc-* | sed -e "s/.*bc-\(.*\).cfg/\1/" )
    testOK runCA authorize co1/private.toml $bcID
    testOK runCA authorize co2/private.toml $bcID
    testOK runCA authorize co3/private.toml $bcID
    # Creat LTS and save the public key
    runCA dkg start --instid "$LTS_ID" -x > key.pub
    
    PUB_KEY=`cat key.pub`
    matchOK $PUB_KEY ^[0-9a-f]{64}$

    # Fail because the Calypso rule "spawn:calypsoWrite" has not been added
    testFail runCA contract write spawn --darc $ID --sign $KEY --instid $LTS_ID --data "Hello world." --key $PUB_KEY

    # Add the missing Calypso rule
    testOK runBA darc rule -rule spawn:calypsoWrite -darc $ID -sign $KEY -identity $KEY
    
    OUTRES=`runCA contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --data "Hello world." --key "$PUB_KEY"`

    matchOK "$OUTRES" "Spawned a new write instance. Its instance id is:
[0-9a-f]{32}"

    # Check the export option
    runCA contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --data "Hello world." --key "$PUB_KEY" -x > iid.txt
    matchOK "`cat iid.txt`" ^[0-9a-f]{64}$
}