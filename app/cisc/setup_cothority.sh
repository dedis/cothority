#!/usr/bin/env bash

# How many nodes to start.
NBR_NODES=3
# Get first non-local available IP address.
IP=$( ifconfig | grep "inet " | grep -v 127.0.0.1 | cut -f 2 -d " " | head -n 1 )

# Compile if binary is not here or if any argument is given.
if [ ! -f cothorityd -o "$1" ]; then
  go build ../cothorityd
fi
if [ ! -f cisc -o "$1" ]; then
  go build
fi

# configure all cothorities.
rm group.toml
killall cothorityd
for n in $( seq $NBR_NODES ); do
  p=$(( 2000 + 2 * n ))
  c=config$n
  rm -rf $c
  echo -e "$p\n$IP\n$c" | ./cothorityd setup
  tail -n 4 $c/group.toml >> group.toml
  # Start cothorityd in background in its own directory.
  ( cd $c; ../cothorityd -c config.toml & )
done

# Create a new identity-skipchain
./cisc id create group.toml
# Print the corresponding qrcode
./cisc id qrcode

echo "Scan QR-Code to join the Skipchain"
read

./cisc config vote
echo "Voted on new device"
read

echo "Add SSH-key to owner"
read

cp -a ~/.ssh ssh-backup.$( date +%s )
./cisc ssh add $HOSTNAME

echo "Accept owner's SSH-key"
read

echo "Add SSH-key"
read
./cisc config vote

echo "Voted on new SSH key"
read

./cisc config list
