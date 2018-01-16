#!/usr/bin/env bash
set -e
(
    cd ../../conode
    conode -d 2 -c co4/private.toml server 2>/tmp/conode4.err 1>/tmp/conode4.log
) &
