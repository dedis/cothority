# Onchain secrets

This is a first implementation of a skipchain that has the following features:

- writing a secret threshold-encrypted to the skipchain
- asking for read-permission and writing the permission to the skipchain

It uses two kind of skipchains:

- acl-skipchain with the following rights:
	- admin: a threshold (for the moment 1) can update the skipchain
	- write: these keys can add new documents to the skipchain
	- read: any public key in here can ask for read access
	
- doc-skipchain with a structure that holds:
	- configuration-block in the genesis-block
		- link to acl-skipchain
	- write-blocks
		- the symmetrically encrypted document ( <10MB)
		- encryption key (will be secret-share encrypted)
	- read-blocks
		- signed request from a reader for a file

To handle the on-chain secrets, two additional methods are available that are
not yet implemented completely:

- EncryptKey: returns a public key with which the writer can encrypt his
encryption key
- DecryptKey: returns the encryption key for the file asymmetrically encrypted using the
reader's public key

## ocsmngr-App

ocsmngr is a minimalistic app that interacts with the onchain-secrets skipchain.
For more information, refer to the README-file in <a href="ocsmngr/README.md">ocsmngr</a>

## Service

The service ensures the correct usage of the skipchains:
- the ACL-skipchain only evolves when a new ACL signed by a previous admin is
proposed
- the Doc-skipchain allows only write-requests from writers in the ACL-skipchain
and read-requests from readers

TODO:

- Implement the EncryptKey and DecryptKey methods to have protected keys.

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

# Testing the local conodes

## Using the example

In the `example`-directory is a complete implementation of the client-side
needed to setup the skipchains, store a document and retrieve it again.
Once the local conodes are running, you can run it:

```bash
go run example/main.go public.toml
```
