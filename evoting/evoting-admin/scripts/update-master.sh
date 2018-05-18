#!/bin/sh

if [ -z "$PIN" ]; then
	echo "PIN env variable is not set"
	exit 1
fi

if [ -z "$CHAIN" ]; then
	echo "CHAIN env variable is not set"
	exit 1
fi

. ./chains/$CHAIN/id.sh

sc_jallen=289938
#sc_jvassalli=140866
sc_lindo=128871
sc_giovanni=121769
admins=$sc_jallen,$sc_lindo,$sc_giovanni

sig=397ffd9cf68fc673dac8451007779a0a59e47b5ebcad69f400c0f07ac7f054a6ea3b6f9b8646280ee37f9960340ae451afe1e703195fb07e59f99fabd3e39404

../evoting-admin \
	-id $id \
	-user $sc_jallen \
	-sig $sig \
	-roster roster.toml \
	-pin $PIN \
	-admins $admins \
	-key 912dd6f8df921f5f51cadc64be3964a6b21a6fe1afac9a7c3b581a45df782895

../evoting-admin \
	-id $id \
	-roster roster.toml \
	-show
