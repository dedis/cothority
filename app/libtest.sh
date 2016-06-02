#!/usr/bin/env bash

# highest number of servers and clients
NBR=3
# Use for suppressing building if that directory exists
STATICDIR=test
# If set, always build
BUILD=
# Debug-level for server
DBG_SRV=1
# Debug-level for client
DBG_CLIENT= 
# Debug running
DBG_RUN=z

startTest(){
    # where the output should go
    if [ "$DBG_RUN" ]; then
        OUT=/dev/stdout
    else
        OUT=/dev/null
    fi
}

test(){
    cleanup
    echo "Testing $1"
    sleep .5
    test$1
}

testOK(){
    if ! $@ > /dev/null; then
        fail "starting $@ failed"
    fi
}

testFail(){
    if $@ > /dev/null; then
        fail "starting $@ failed"
    fi
}

testFile(){
    if [ ! -f $1 ]; then
        fail "file $1 is not here"
    fi
}

testGrep(){
    S=$1
    shift
    if ! $@ | grep -q "$S"; then
        fail "Didn't find '$S' in output of '$@'"
    fi
}

testNGrep(){
    S=$1
    shift
    if $@ | grep -q "$S"; then
        fail "Found '$S' in output of '$@'"
    fi
}

dbgRun(){
    if [ "$DBG_RUN" ]; then
        echo $@
    fi
}

fail(){
    echo
    echo -e "\tFAILED: $@"
    cleanup
    exit 1
}

cleanup(){
    pkill cosi 2> /dev/null
    pkill ssh-ks 2> /dev/null
    pkill cothorityd 2> /dev/null
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
