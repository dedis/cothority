# Binary distribution of conode

This is the binary distribution of the conode for building a cothority with your 
friends. It contains 32/64 bit binaries for linux and MacOSX-computers.

## Setup

Each conode needs to create it's own private/public key pair. For this, you can
run it with

```
./start-conode setup IP-address:port
```

Where `IP-address` is your publicly reachable IP-address and `port` and
`port + 1` must be open for incoming packets.
 
The public-key is automatically sent to the EPFL-conode team.

## Run your own conodes

If you want to run your own conodes, you have to do the `setup`-step on each
computer where you want to run your conode. Then you have to collect all public
keys and name them `key-id.pub` where `id` is an identifier you can chose your
own.

All files have to be in the `keys`-directory.

ATTENTION: as of now, the conodes search for an update in the github-directory.
To not let the updates happen (which overwrite also your definition of the tree),
create an empty file called `NO_UPDATES` in this directory.

### Verify and restart conodes

It is best to start to verify the availability of the conodes by using

```
./conode check keys/key-id.pub
```

where `id` is again one of the ids you distributed above. If all nodes are up
and verified with success, you have to copy the `config.toml` to all conodes.
Then you can run

```
./conode check exit keys/key-id.pub
```

for all conodes, so that they get started again with the new configuration.

### Test the conodes

You can test everything is running correctly using the

```
./check_stampers.sh
```

which will contact all conodes in turn and get a stamp.
