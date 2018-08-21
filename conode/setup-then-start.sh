#!/bin/sh

if [ ! -f /conode_data/private.toml ]; then
    ./conode setup --non-interactive
fi

./conode -debug 2 server
