#!/usr/bin/env bash

SERVER_GW="root@$1"
SERVERS="$@"
KEYS=/tmp/server_keys

rm $KEYS
for s in $SERVERS; do
	ssh-copy-id root@$s
	ssh root@$s cat .ssh/id_rsa.pub >> $KEYS
	scp install_mininet.sh root@$s
	ssh root@$s "nohup . ./install_mininet.sh &"
done

echo -n "Press <enter> when mininet is installed"
read

for s in $SERVERS; do
	cat $KEYS | ssh $SERVER_GW "cat - >> .ssh/authorized_keys"
done
