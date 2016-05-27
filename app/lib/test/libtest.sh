#!/usr/bin/env bash

# highest number of servers and clients
NBR=3
# Use for suppressing building if that directory exists
STATICDIR=test
# If set, always build
BUILD=
# Show the output of the commands
DBG_SHOW=0

startTest(){
    set +m
}

test(){
    cleanup
    echo "Testing $1"
    sleep .5
    test$1
}

testOK(){
    testOut "Assert OK for '$@'"
    if ! dbgRun "$@"; then
        fail "starting $@ failed"
    fi
}

testFail(){
    testOut "Assert FAIL for '$@'"
    if dbgRun "$@"; then
        fail "starting $@ should've failed, but succeeded"
    fi
}

testFile(){
    if [ ! -f $1 ]; then
        fail "file $1 is not here"
    fi
}

testGrep(){
    S="$1"
    shift
    testOut "Assert grepping '$S' in '$@'"
    runGrep "$S" "$@"
    if [ ! "$GRP" ]; then
        fail "Didn't find '$S' in output of '$@'"
    fi
}

testNGrep(){
    S="$1"
    shift
    testOut "Assert NOT grepping '$S' in '$@'"
    runGrep "$S" "$@"
    if [ "$GRP" ]; then
        fail "Did find '$S' in output of '$@'"
    fi
}

testOut(){
    if [ "$DBG_SHOW" -ge 1 ]; then
        echo -e "$@"
    fi
}

dbgOut(){
    if [ "$DBG_SHOW" -ge 2 ]; then
        echo -e "$@"
    fi
}

dbgRun(){
    if [ "$DBG_SHOW" -ge 2 ]; then
        OUT=/dev/stdout
    else
        OUT=/dev/null
    fi
    if [ "$GREP" ]; then
        $@ 2>&1 | tee $GREP > $OUT
    else
        $@ 2>&1 > $OUT
    fi
}

runSed(){
    SED="$1"
    shift
    OLDGREP=$GREP
    GREP=$( mktemp )
    dbgRun "$@"
    SED=$( cat $GREP | sed -e "$SED" )
    GREP=$OLDGREP
}

runGrep(){
    GRP="$1"
    shift
    OLDGREP=$GREP
    GREP=$( mktemp )
    dbgRun "$@"
    GRP=$( cat $GREP | egrep "$GRP" )
    GREP=$OLDGREP
}

fail(){
    echo
    echo -e "\tFAILED: $@"
    cleanup
    exit 1
}

backg(){
    ( $@ 2>&1 & )
}

cleanup(){
    pkill cothorityd 2> /dev/null
    pkill cosi 2> /dev/null
    pkill ssh-ks 2> /dev/null
    sleep .5
    rm -f srv*/*bin
    rm -f cl*/*bin
}

stopTest(){
    cleanup
    if [ ! "$STATICDIR" ]; then
        echo "removing $DIR"
        rm -rf $DIR
    fi
    echo "Success"
}
