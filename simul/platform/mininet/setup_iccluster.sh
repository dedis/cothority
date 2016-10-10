#!/usr/bin/env bash

ICCLUSTERS=""
for s in $@; do
  ICCLUSERS="$ICCLUSTERS iccluster${s}.iccluster.epfl.ch"
done

./setup_servers.sh $ICCLUSTERS
