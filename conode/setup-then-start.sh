#!/bin/sh

if [ ! -f /conode_data/private.toml ]; then
    ./conode setup --non-interactive
fi

DEBUG_TIME=true ./conode -debug 2 server
