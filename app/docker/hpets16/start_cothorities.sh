#!/bin/bash
killall cothorityd 2&> /dev/null
sleep 1
for s in 0 1 2; do
  ( cothorityd -d 2 -c bin/config/server$s/config.toml & )
done
cosi check
