# Onchain secrets

This is a first implementation of a skipchain that has the following features:

- writing a secret threshold-encrypted to the skipchain
- asking for read-permission and writing the permission to the skipchain

The skipchain has the following transactions:
- write-blocks
	- the symmetrically encrypted document ( <10MB)
	- encryption key (secret-share encrypted)
- read-blocks
	- signed request from a reader for a file
- readers
    - a list of readers that are allowed to access a file. Either a
    static list, or a modifiable list that can be updated by
    one or more administrators

## ocsmgr-App

ocsmgr is a minimalistic app that interacts with the onchain-secrets skipchain.
For more information, refer to the README-file in <a href="ocsmgr/README.md">ocsmgr</a>

## Service

The service ensures the correct usage of the skipchains and offers an
API to the OCS-protocols:

- creating a skipchain
- writing an encrypted symmetric key and a file
- create a read request
- get public key of the Distributed Key Generator (DKG)
- get all read requests

# Starting a local set of test-nodes

## Using run_conode.sh

To start a set of local conodes that store documents, simply run:

```bash
conode/run_conode.sh local 3 2
```

The number `3` indicates how many nodes should be started, and the number `2`
indicates the debug-level: `0` is silent, and `5` is very verbose.
 
To stop the running nodes, use

```bash
pkill -f conode
```

## Docker-files

The Dockerfile can be used for putting the nodes in a docker-container. You
can build it using:

```bash
make docker
```

Once it's built, run it with

```bash
make docker_run
```

# Interacting with the test-nodes

The easiest way to interact with the test-nodes is to use the ocsmgr at
[ocsmgr/README.md].