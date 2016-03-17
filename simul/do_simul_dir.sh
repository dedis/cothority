#!/usr/bin/env bash

PLATFORM="deterlab"

if [ -z "$1" ]; then
    echo "Syntax: $0 directory [flags]"
    exit 1
fi

if [ ! -d $1 ]; then
    echo "Directory $0 doesn't exist"
    exit 1
fi

DIR="$1"
shift
FLAGS="$@"
if [ "$FLAGS" ]; then
    GREP=""
else
    GREP="(Starting run with parameters|^F :|^E :|^W :)"
fi
NOBUILD=""
go build

for simul in $DIR/*toml; do
    echo "Simulating file $(basename $simul)"
    ./simul -platform $PLATFORM $NOBUILD $FLAGS $simul | egrep "$GREP"
    NOBUILD="-nobuild"
done
