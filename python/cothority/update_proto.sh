#!/usr/bin/env bash

INCLUDE="-I ../../../cothority -I ../../../onet"
ONET="network/network.proto messages.proto"
COTHORITY="status/status.proto skipchain/skipchain.proto"
PROTOS=""

for p in $ONET; do
	PROTOS="$PROTOS ../../../onet/$p"
done
for p in $COTHORITY; do
	PROTOS="$PROTOS ../../../cothority/$p"
done

echo $PROTOS
protoc $INCLUDE --python_out=. $PROTOS

