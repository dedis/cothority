#!/usr/bin/env bash

# Usage: 
#   ./test [options]
# Options:
#   -b   re-builds bcadmin package

DBG_TEST=1
DBG_SRV=2
DBG_APP=2

NBR_SERVERS=4
NBR_SERVERS_GROUP=3

# Clears some env. variables
export -n BC_CONFIG
export -n BC
export BC_WAIT=true

. "../../libtest.sh"
. "../clicontracts/lts_test.sh"
. "../clicontracts/write_test.sh"
. "../clicontracts/read_test.sh"

main(){
    startTest
    buildConode go.dedis.ch/cothority/v3/calypso
    build $APPDIR/../../byzcoin/bcadmin
    run testAuth
    run testContractLTS
    run testDkgStart
    run testContractWrite
    run testContractRead
    run testReencrypt
    run testDecrypt
    stopTest
}

testAuth(){
    rm -f config/*
    runCoBG 1 2 3
    runBA create public.toml --interval .5s
    bcID=$( ls config/bc-* | sed -e "s/.*bc-\(.*\).cfg/\1/" )

    testFail runCA authorize
    testFail runCA authorize co2/private.toml

    # Create hybrid private with private key from wrong node
    cp co1/private.toml private_wrong.toml
    PRIV2=$( egrep "^Private =" co2/private.toml | sed -e 's/.*"\(.*\)"/\1/' )
    perl -pi -e "s/^Private.*/Private = \"$PRIV2\"/" private_wrong.toml
    testFail runCA authorize private_wrong.toml $bcID

    # Correct signature
    testOK runCA authorize co1/private.toml $bcID

    # Test with signature check disabled
    pkill conode 2> /dev/null
    export COTHORITY_ALLOW_INSECURE_ADMIN=true
    runCoBG 1 2 3
    # Because the old bcID is already stored, create a new one
    # after cleaning the first one
    testOK rm config/bc-*.cfg
    runBA create public.toml --interval .5s
    bcID=$( ls config/bc-* | sed -e "s/.*bc-\(.*\).cfg/\1/" )
    testOK runCA authorize private_wrong.toml $bcID

    # It is important to unset it because when it's true a message log is
    # printed each time and that invalidates some of our tests that checks the
    # output.
    unset COTHORITY_ALLOW_INSECURE_ADMIN
}

# Rely on `csadmin contract lts spawn` and `csadmin authorize`
testDkgStart(){
    rm -f config/*
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    OUTRES=`runCA0 contract lts spawn`
    LTS_ID=`echo "$OUTRES" | sed -n '2p'`
    matchOK $LTS_ID "^[0-9a-f]{64}$"

    # no --instid
    testFail runCA dkg start
    # wrong --instid
    testFail runCA dkg start --instid aef123
    # good --instid but this byzcoin has not been authorised
    testFail runCA dkg start --instid "$LTS_ID"
    
    # let's make it pass with `csadmin authorize`
    bcID=$( ls config/bc-* | sed -e "s/.*bc-\(.*\).cfg/\1/" )
    testOK runCA authorize co1/private.toml $bcID
    testOK runCA authorize co2/private.toml $bcID
    testOK runCA authorize co3/private.toml $bcID
    testOK runCA dkg start --instid "$LTS_ID"

    testGrep "LTS created:
- ByzcoinID: [0-9a-f]{64}
- InstanceID: [0-9a-f]{64}
- X: [0-9a-f]{64}$" runCA dkg start --instid "$LTS_ID"

    # Check the --export option
    testGrep "[0-9a-f]{64}$" runCA dkg start --instid "$LTS_ID" -x
}

# rely on:
# - csadmin contract lts spawn
# - csadmin authorize
# - csadmin contract write spawn
# - csadmin contract read spawn
testReencrypt(){
    rm -f config/*
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Create a DARC
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:longTermSecret" --darc $ID --sign $KEY --identity $KEY
    testOK runBA darc rule -rule "spawn:calypsoWrite" -darc $ID -sign $KEY -identity $KEY
    testOK runBA darc rule -rule "spawn:calypsoRead" -darc $ID -sign $KEY -identity $KEY

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
    
    # Spawn write
    OUTRES=`runCA0 contract write spawn --darc "$ID" --sign "$KEY"\
                    --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY"`
    WRITE_ID=`echo "$OUTRES" | sed -n '2p'` # must be at the second line
    matchOK $WRITE_ID ^[0-9a-f]{64}$

    # Spawn read
    OUTRES=`runCA0 contract read spawn --sign $KEY --instid $WRITE_ID`
    READ_ID=`echo "$OUTRES" | sed -n '2p'` # must be at the second line
    matchOK $READ_ID ^[0-9a-f]{64}$

    # a wrong readid
    testFail runCA decrypt --writeid $WRITE_ID --readid aef123

    # a wrong writeid
    testFail runCA decrypt --writeid aef123 --readid $READ_ID

    # Run a reencrypt and check the output
    OUTRES=`runCA0 reencrypt --writeid $WRITE_ID --readid $READ_ID`
    # OUTRES=`echo "$OUTRES" | tr -d '\n'`
    matchOK "$OUTRES" "Got decrypt reply:
- C: [0-9a-f]{64}
- xHat: [0-9a-f]{64}
- X: [0-9a-f]{64}"

    # Check if the --export option is okay. We will check the exported content
    # while using `csadmin recover`.
    testOK runCA reencrypt --writeid $WRITE_ID --readid $READ_ID -x
}

# rely on:
# - csadmin contract lts spawn
# - csadmin authorize
# - csadmin contract write spawn
# - csadmin contract read spawn
# - csadmin decrypt
# - bcadmin key
testDecrypt(){
    rm -f config/*
    runCoBG 1 2 3
    runGrepSed "export BC=" "" runBA create --roster public.toml --interval .5s
    eval $SED
    [ -z "$BC" ] && exit 1

    # Create a DARC
    testOK runBA darc add -out_id ./darc_id.txt -out_key ./darc_key.txt -unrestricted
    ID=`cat ./darc_id.txt`
    KEY=`cat ./darc_key.txt`
    testOK runBA darc rule -rule "spawn:longTermSecret" --darc $ID --sign $KEY --identity $KEY
    testOK runBA darc rule -rule "spawn:calypsoWrite" -darc $ID -sign $KEY -identity $KEY
    testOK runBA darc rule -rule "spawn:calypsoRead" -darc $ID -sign $KEY -identity $KEY

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
    
    # Spawn write
    OUTRES=`runCA0 contract write spawn --darc "$ID" --sign "$KEY" \
                    --instid "$LTS_ID" --secret "Hello world." --key "$PUB_KEY"`
    WRITE_ID=`echo "$OUTRES" | sed -n '2p'` # must be at the second line
    matchOK $WRITE_ID ^[0-9a-f]{64}$

    # Spawn read
    OUTRES=`runCA0 contract read spawn --sign $KEY --instid $WRITE_ID`
    READ_ID=`echo "$OUTRES" | sed -n '2p'` # must be at the second line
    matchOK $READ_ID ^[0-9a-f]{64}$

    # run the reencrypt and save the reply
    runCA0 reencrypt --writeid $WRITE_ID --readid $READ_ID -x > reply.bin

    # should fail since it will use the default key, the admin one, and this is
    # not the key that was set for the read request
    OUTRES=`runCA decrypt < reply.bin`
    testNGrep "Hello world" echo "$OUTRES"

    # should pass with the correct --key
    OUTRES=`runCA0 decrypt --key config/key-$KEY.cfg < reply.bin`
    matchOK "$OUTRES" "Key decrypted:
Hello world."

    # Check the export option
    runCA0 decrypt --key config/key-$KEY.cfg -x < reply.bin > data.txt
    matchOK "`cat data.txt`" "Hello world."

    #
    # Now lets try to generate a new key and use this one to encrypt the data:
    #
    #  1: generate a new key
    #  2: create a new read request
    #  3: get re-encrypted data based on the read and write requests
    #  4: recover the re-encrypted data with the key

    # 1:
    NEW_KEY=`runBA key`
    matchOK $NEW_KEY "ed25519:([0-9a-f]{64})"
    # We capture the regex group () in order to get only the hex public key
    # string. The following won't work with zsh.
    PUB_KEY=${BASH_REMATCH[1]}
    matchOK $PUB_KEY "[0-9a-f]{64}"

    # 2:
    OUTRES=`runCA0 contract read spawn --sign $KEY --instid $WRITE_ID --key $PUB_KEY`
    READ_ID=`echo "$OUTRES" | sed -n '2p'` # must be at the second line
    matchOK $READ_ID ^[0-9a-f]{64}$

    # 3:
    runCA0 reencrypt --writeid $WRITE_ID --readid $READ_ID -x > reply.bin

    # 4:

    # should fail with the default key
    OUTRES=`runCA decrypt < reply.bin`
    testNGrep "Hello world" echo "$OUTRES"
    # should fail with the key used to sign
    OUTRES=`runCA decrypt --key config/key-$KEY.cfg < reply.bin`
    testNGrep "Hello world" echo "$OUTRES"
    
    # should now work with the newly created key
    OUTRES=`runCA0 decrypt --key config/key-$NEW_KEY.cfg < reply.bin`
    matchOK "$OUTRES" "Key decrypted:
Hello world."
}

runCA(){
    ./csadmin -c config/ --debug $DBG_APP "$@"
}

runCA0(){
    ./csadmin -c config/ --debug 0 "$@"
}

runBA(){
    ./bcadmin -c config/ --debug $DBG_APP "$@"
}

main
