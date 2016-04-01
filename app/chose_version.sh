#!/usr/bin/env bash
# Checks which binary should be run and gives an error if used in wrong
# context
NAME=$(basename $0)
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
if [ $NAME = "chose_version.sh" ]; then
    echo "This should not be run directly"
    exit 1
fi

if uname -a | grep -q Darwin; then
    OS=darwin
else
    OS=linux
fi
BIN=$DIR/$NAME-${OS}-amd64
if [ -f $bin ]; then
    $BIN $@
else
    echo "$BIN is not available - do you run it from the pre-compiled tar.gz?"
    exit 1
fi
