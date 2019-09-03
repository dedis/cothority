# This method should be called from the calypso/csadmin/test.sh script

testContractWrite() {
    run testContractWriteInvoke
    run testContractWriteGet
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
    OUTRES=`runCA0 contract lts spawn --darc "$ID" --sign "$KEY"`
    LTS_ID=`echo "$OUTRES" | sed -n '2p'`
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

    # Fail because the Calypso rule "spawn:calypsoWrite" has not been added
    testFail runCA contract write spawn --darc $ID --sign $KEY --instid $LTS_ID --secret "Hello world." --key $PUB_KEY

    # Add the missing Calypso rule
    testOK runBA darc rule -rule spawn:calypsoWrite -darc $ID -sign $KEY -identity $KEY
    
    OUTRES=`runCA0 contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY"`

    matchOK "$OUTRES" "^Spawned a new write instance. Its instance id is:
[0-9a-f]{64}$"

    # We check only that commands exits correctly. The content should be checked
    # by a `csadmin contract write get` function.

    # Check the export option
    runCA0 contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY" -x > iid.txt
    matchOK "`cat iid.txt`" ^[0-9a-f]{64}$

    # Check the --data option
    testOK runCA contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY" --data "This should be encrypted data"

    # Check the --readData option
    testOK echo "This should be encrypted data" | runCA contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY" --readData

    # Check the --readData option with --data
    testOK echo "This should be encrypted data" | runCA contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY" --readData --data "not used"

    # Check the --extraData option
    testOK runCA contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY" --extraData "This is some cleartext data"

    # Check the --readExtra option
    testOK echo "This is some cleartext data" | runCA contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY" --readExtra

    # Check the --readExtra option with --extraData
    testOK echo "This is some cleartext data" | runCA contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY" --readExtra --extraData "not used"

    # Using --readData and --readExtra should not be permitted
    testFail runCA contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY" --readExtra --readData
    testFail runCA contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY" --readExtra --readData --extraData "extra" --data "data"
}

# rely on:
# - csadmin contract lts spawn
# - csadmin authorize
testContractWriteGet() {
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
    LTS_ID=`echo "$OUTRES" | sed -n '2p'`
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

    # Add the missing Calypso rule
    testOK runBA darc rule -rule spawn:calypsoWrite -darc $ID -sign $KEY -identity $KEY
    
    runCA0 contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID"\
                    --secret "Hello world." --key "$PUB_KEY"\
                    --data "Should be encrypted" -x > writeid.txt
    WRITE_ID=`cat writeid.txt`
    matchOK $WRITE_ID ^[0-9a-f]{64}$

    # Lets now check the result
    OUTRES=`runCA0 contract write get --instid $WRITE_ID`

    matchOK "$OUTRES" "^- Write:
-- Data: Should be encrypted
-- U: [0-9a-f]{64}
-- Ubar: [0-9a-f]{64}
-- E: [0-9a-f]{64}
-- F: [0-9a-f]{64}
-- C: [0-9a-f]{64}
-- ExtraData: 
-- LTSID: [0-9a-f]{64}
-- Cost: .*$"

    # Use the --readin option
    echo -n "Should be encrypted - from STDIN" | runCA0 contract write spawn\
                    --darc "$ID" --sign "$KEY" --instid "$LTS_ID"\
                    --secret "Hello world." --key "$PUB_KEY"\
                    --readData -x > writeid.txt
    WRITE_ID=`cat writeid.txt`
    matchOK $WRITE_ID ^[0-9a-f]{64}$

    # Lets now check the result
    OUTRES=`runCA0 contract write get --instid $WRITE_ID`

    matchOK "$OUTRES" "^- Write:
-- Data: Should be encrypted - from STDIN
-- U: [0-9a-f]{64}
-- Ubar: [0-9a-f]{64}
-- E: [0-9a-f]{64}
-- F: [0-9a-f]{64}
-- C: [0-9a-f]{64}
-- ExtraData: 
-- LTSID: [0-9a-f]{64}
-- Cost: .*$"

    # Provide no data
    runCA0 contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID"\
                    --secret "Hello world." --key "$PUB_KEY" -x > writeid.txt
    WRITE_ID=`cat writeid.txt`
    matchOK $WRITE_ID ^[0-9a-f]{64}$

    # Lets now check the result
    OUTRES=`runCA0 contract write get --instid $WRITE_ID`

    matchOK "$OUTRES" "^- Write:
-- Data: 
-- U: [0-9a-f]{64}
-- Ubar: [0-9a-f]{64}
-- E: [0-9a-f]{64}
-- F: [0-9a-f]{64}
-- C: [0-9a-f]{64}
-- ExtraData: 
-- LTSID: [0-9a-f]{64}
-- Cost: .*$"

    # Provide both --data and --readin. --readin should be used.
    echo -n "Should be encrypted - from STDIN" | runCA0 contract write spawn\
                    --darc "$ID" --sign "$KEY" --instid "$LTS_ID"\
                    --secret "Hello world." --key "$PUB_KEY"\
                    --readData --data "Hello there." -x > writeid.txt
    WRITE_ID=`cat writeid.txt`
    matchOK $WRITE_ID ^[0-9a-f]{64}$

    # Lets now check the result
    OUTRES=`runCA0 contract write get --instid $WRITE_ID`

    matchOK "$OUTRES" "^- Write:
-- Data: Should be encrypted - from STDIN
-- U: [0-9a-f]{64}
-- Ubar: [0-9a-f]{64}
-- E: [0-9a-f]{64}
-- F: [0-9a-f]{64}
-- C: [0-9a-f]{64}
-- ExtraData: 
-- LTSID: [0-9a-f]{64}
-- Cost: .*$"

    # Provide both --data and --extraData.
    runCA0 contract write spawn --darc "$ID" --sign "$KEY" --instid "$LTS_ID"\
                    --secret "Hello world." --key "$PUB_KEY"\
                    --data "Should be encrypted"\
                    --extraData "Public infos" -x > writeid.txt
    WRITE_ID=`cat writeid.txt`
    matchOK $WRITE_ID ^[0-9a-f]{64}$

    # Lets now check the result
    OUTRES=`runCA0 contract write get --instid $WRITE_ID`

    matchOK "$OUTRES" "^- Write:
-- Data: Should be encrypted
-- U: [0-9a-f]{64}
-- Ubar: [0-9a-f]{64}
-- E: [0-9a-f]{64}
-- F: [0-9a-f]{64}
-- C: [0-9a-f]{64}
-- ExtraData: Public infos
-- LTSID: [0-9a-f]{64}
-- Cost: .*$"

    # Provide both --extraData and --readExtra. --readExtra should be used.
    echo -n "Extra data - from STDIN" | runCA0 contract write spawn\
                    --darc "$ID" --sign "$KEY" --instid "$LTS_ID"\
                    --secret "Hello world." --key "$PUB_KEY"\
                    --readExtra --extraData "Hello there." -x > writeid.txt
    WRITE_ID=`cat writeid.txt`
    matchOK $WRITE_ID ^[0-9a-f]{64}$

    # Lets now check the result
    OUTRES=`runCA0 contract write get --instid $WRITE_ID`

    matchOK "$OUTRES" "^- Write:
-- Data: 
-- U: [0-9a-f]{64}
-- Ubar: [0-9a-f]{64}
-- E: [0-9a-f]{64}
-- F: [0-9a-f]{64}
-- C: [0-9a-f]{64}
-- ExtraData: Extra data - from STDIN
-- LTSID: [0-9a-f]{64}
-- Cost: .*$"

    # Provide both --data and --readExtra.
    echo -n "Extra data - from STDIN" | runCA0 contract write spawn\
                    --darc "$ID" --sign "$KEY" --instid "$LTS_ID"\
                    --secret "Hello world." --key "$PUB_KEY"\
                    --readExtra --data "Hello there." -x > writeid.txt
    WRITE_ID=`cat writeid.txt`
    matchOK $WRITE_ID ^[0-9a-f]{64}$

    # Lets now check the result
    OUTRES=`runCA0 contract write get --instid $WRITE_ID`

    matchOK "$OUTRES" "^- Write:
-- Data: Hello there.
-- U: [0-9a-f]{64}
-- Ubar: [0-9a-f]{64}
-- E: [0-9a-f]{64}
-- F: [0-9a-f]{64}
-- C: [0-9a-f]{64}
-- ExtraData: Extra data - from STDIN
-- LTSID: [0-9a-f]{64}
-- Cost: .*$"

    # Provide both --extraData and --readData.
    echo -n "Should be encrypted - from STDIN" | runCA0 contract write spawn\
                    --darc "$ID" --sign "$KEY" --instid "$LTS_ID"\
                    --secret "Hello world." --key "$PUB_KEY"\
                    --readData --extraData "Hello there." -x > writeid.txt
    WRITE_ID=`cat writeid.txt`
    matchOK $WRITE_ID ^[0-9a-f]{64}$

    # Lets now check the result
    OUTRES=`runCA0 contract write get --instid $WRITE_ID`

    matchOK "$OUTRES" "^- Write:
-- Data: Should be encrypted - from STDIN
-- U: [0-9a-f]{64}
-- Ubar: [0-9a-f]{64}
-- E: [0-9a-f]{64}
-- F: [0-9a-f]{64}
-- C: [0-9a-f]{64}
-- ExtraData: Hello there.
-- LTSID: [0-9a-f]{64}
-- Cost: .*$"
}