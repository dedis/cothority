#!/usr/bin/env bash

# Creates a tree out of all keys/key*pub files. It also copies the config.toml
# to the stamp-directory, if it is available.

cat keys/key*pub > keys/hostlist
./conode build keys/hostlist
cp config.toml keys
if [ -d stamp ]; then
  cp config.toml stamp
fi
echo Built new tree - restart nodes