#!/bin/sh

if [ -z "$CHAIN" ]; then
	echo "CHAIN env variable is not set."
	exit 1
fi

. ./chains/$CHAIN/id.sh

./evoting-admin \
	-id $id \
	-roster leader.toml \
	-show
