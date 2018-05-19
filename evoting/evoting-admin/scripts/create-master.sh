#!/bin/sh

if [ -z "$PIN" ]; then
        echo "PIN env variable is not set"
        exit 1
fi

sc_jallen=289938
sc_lindo=128871
sc_giovanni=121769
admins=$sc_jallen,$sc_lindo,$sc_giovanni

../evoting-admin \
	-roster roster.toml \
	-pin $PIN \
	-admins $admins
