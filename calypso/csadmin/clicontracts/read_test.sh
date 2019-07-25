# This method should be called from the calypso/csadmin/test.sh script

testContractRead() {
    run testContractReadInvoke
}

# rely on:
# - csadmin contract lts spawn
# - csadmin authorize
# - csadmin contract write spawn
testContractReadInvoke() {
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
    OUTRES=`runCA0 contract lts spawn --darc "$ID" --sign "$KEY"`
    LTS_ID=`echo "$OUTRES" | sed -n '2p'` # must be at the second line
    matchOK $LTS_ID ^[0-9a-f]{64}$
    # Authorize nodes
    bcID=$( ls config/bc-* | sed -e "s/.*bc-\(.*\).cfg/\1/" )
    testOK runCA authorize co1/private.toml $bcID
    testOK runCA authorize co2/private.toml $bcID
    testOK runCA authorize co3/private.toml $bcID
    # Creat LTS and save the public key
    runCA0 dkg start --instid "$LTS_ID" -x > key.pub
    PUB_KEY=`cat key.pub`
    matchOK $PUB_KEY ^[0-9a-f]{64}$

    # Add the Calypso rule "spawn:calypsoWrite"
    testOK runBA darc rule -rule spawn:calypsoWrite -darc $ID -sign $KEY -identity $KEY
    
    OUTRES=`runCA0 contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY"`
    WRITE_ID=`echo "$OUTRES" | sed -n '2p'` # must be at the second line

    # Should fail because we miss the "spawn:calypsoRead" rule
    testFail runCA contract read spawn --sign $KEY --instid $WRITE_ID

    # Add the Calypso rule
    testOK runBA darc rule -rule spawn:calypsoRead -darc $ID -sign $KEY -identity $KEY

    OUTRES=`runCA0 contract read spawn --sign $KEY --instid $WRITE_ID`

    matchOK "$OUTRES" "^Spawned a new read instance. Its instance id is:
[0-9a-f]{64}$"

    # Check the export option
    runCA0 contract read spawn --sign $KEY --instid $WRITE_ID -x > iid.txt
    matchOK "`cat iid.txt`" ^[0-9a-f]{64}$
}