#!/bin/sh

if [ -z "$PIN" ]; then
        echo "PIN env variable is not set"
        exit 1
fi

if [ -z "$1" ]; then
	echo "Need roster.toml as first argument."
	exit 1
fi

sc_jallen=289938
sc_lindo=128871
sc_giovanni=121769
sc_nkcr=228271
admins=$sc_jallen,$sc_lindo,$sc_giovanni,$sc_nkcr

evoting-admin \
	-roster $1 \
	-pin $PIN \
	-admins $admins
