Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Operating a Conode

# Operating a Conode

The conode can be installed and started like this:

```
go get github.com/dedis/cothority
cd $(go env GOPATH)/src/github.com/dedis/cothority/conode
./run_conode.sh public
```

The first time you run `run_conode.sh public` it will set up a new conode.
Subsequent times, it starts the server, logging to stdout. You can run it in a
screen session to make it survive logouts.

You should run it as a non-root user. It can also be run from a Docker image
(documentation to come, when Dockerfile is merged from the other repository to
here).

For more information about how to run a conode on the command line, see
[Running a Conode](RunningConode.md).

## Configuration

The setup routine writes the config files into `$HOME/.config/conode` on Linux
(`$HOME/Library/conode` on MacOS, `%USERPROFILE%/AppData/Local/Conode` on Windows).

The `private.toml` file holds the server config, including the private key. It
also includes the server's public address on the network. The server will listen
to this port, as well as to this port + 1 (for websocket connections).

# Reverse proxy

Conode should only be run as a non-root user.

The current recommended way to add HTTPS to the websocket port is to use a web
server like Apache or nginx in reverse proxy mode to forward connections from
port 443 to port 6880, the default websocket port.

If you want that the websocket port is on a port under 1024 (i.e. 443 for
HTTPS), you can use setcap to give the conode binary the necessary privs: `sudo
setcap CAP_NET_BIND_SERVICE=+eip $(go env GOPATH)/bin/conode`

# Backups

On Linux, the following files need to be backed up:
1. `$HOME/.config/conode/private.toml`
2. `$HOME/.local/share/conode/$PUBLIC_KEY.db`

The DB file is a BoltDB file, and more information about considerations while
backing them up is in
[Database backup](https://github.com/dedis/onet/wiki/Database-backup-and-recovery).

# Recovery from a crash

If you have a backup of the private.toml file and a recent backup of the .db
file, you can put them onto a new server, and start the conode. The IP address
in the private.toml file must match the IP address on the server.

## Roster IPs should be movable

In order to facilitate IP address switches, it is recommended that the public IP
address for the leader of critical skipchains should be a virtual address. For
example, if you have two servers:
* 10.0.0.2 conode-live, also with secondary address 10.0.0.1
* 10.0.0.3 conode-standby

You can keep both servers running, and use scp to move the DB file from
conode-live to conode-standby. Both servers should have the same private.toml
file, which includes the line `Address = "tcp://10.0.0.1:6879"`

In the event that conode-live is down and unrecoverable, you can add 10.0.0.1 as
a secondary address to conode-standby and start the conode on it. From this
moment on, you must be sure that conode-live does not boot, or if it does, that
it *does not* have the secondary address on it anymore. You could do so by not
adding the secondary address to boot-time configs, and only move it manually.

The address 10.0.0.1 will be in the Roster of any skipchains, and nodes which
are following that skipchain will still be able to contact the leader, even if
it is now running on a different underlying server.

Note: The address part of a server identity has name resolution applied to it.
Thus it would be possible to set the roster of a skipchain to include a server
identity like "tcp://conode-master.example.com:6979" and then change the
definition of conode-master.example.com in DNS in order to change the IP address
of the master.

## Catch-up Behavior

(This section applies to the [Skipchain Service](../skipchain/README.md),
and to any service which depends on Skipchains.)

If the conode is a follower for a given skipchain, then when it is asked to add
a block onto that skipchain by the leader, it will contact other nodes in the
last known roster for that skipchain in order to get copies of blocks that it is
missing. Once it has followed the skipchain to the latest block mentioned in the
proposed update, it will add the proposed block.

If the conode is a leader on a skipchain, when it is asked to add a block with a
latest block id that it does not know, it will attempt to catch up from other
conodes in the last known roster of the skipchain. If it can find the latest
block requested by the client from another member of the cothority, then it will
catch up to that block, and start the process of adding the block (i.e. request
that other conodes sign the block, and sign forward links to it, adding the
block into the chain). If the client proposes a block for a skipchain that is
not known to the conode, it will not attempt to catch up with other conodes.

Thus, it is imperative that the leader's DB is backed up regularly. Even though
it is possible that the leader can recover from peers, genesis blocks (which
start new skipchains) can *only* be backed up via out-of-band methods of
protecting the integrity of the leader's DB file.

## Reverting the cothority

If you control all of the nodes in a cothority, it is in theory possible to
rewrite history by saving all the DB files at a certain point in time, and then
later restoring them all at the same time. To be certain that all the DB files
are consistent with themselves, and with each other, all of the conodes should
be down at the time the backup is taken.

If you do not control more than 2/3 of the conodes, it is not possible to rewind
the state of the cothority. This is by design, and is the fundamental feature of
a group of mutually untrusted servers who are working together to provide a
cothority.
