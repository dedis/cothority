# Onchain secrets

This is a first implementation of a skipchain that implements the
onchain-secrets protocol as developed by DEDIS/EPFL. It allows the
storage of encrypted data on the skipchain with a read-access list
and then re-encrypts that data so that only one of the readers can
have access to it. In such a way the access to the data is always
logged, and eventual leakage can be tracked, or a payment system can
be set up. Additionally, the list of readers can be updated after the
write of the encrypted data, so that changing groups of people can access
the data, or that access can be revoked.

## Basic Workflow

This is how onchain-secrets work:

1. Setup
    - an **administrator** asks a **cothority** to set up a skipchain
    and to perform a Distributed Key Generation (DKG)
    - the cothority returns the public aggregate key _X_

2. Writing
    - a **writer** choses a random symmetric key _k_ and uses it to encrypt
    a document _d_ to produce _d_enc_
    - the writer encrypts the symmetric key using _X_ of the cothority
    to produce _k_enc_
    - _d_enc_, _k_enc_, and the list of authorized readers are stored on the
    skipchain

3. Reading
    - a **reader** sends a read request to the cothority and authenticates
    by signing the write-id of the document
    - the cothority verifies the authentication and adds the read request
    as a new block if the authentication succeeds
    - the reader requests a re-encryption of _k_enc_ under the reader's
    public key and receives _k_enc_reader_
    - using his private key, the reader can recover _k_. From the skipchain
    he can get _d_enc_ and recover the original document _d_

4. Auditing
    - an **auditor** can traverse the skipchain and see when a reader
    accessed a certain document.

## App

The OnChain Manager (ocsmgr) is a text-based app that interacts with the
onchain-secrets skipchain. You can find more information in its directory
at [ocsmgr/README.md]

# Repository

This repository holds the protocol for Distributed Key Generation and
for the OnChain-Secret key re-encryption. It also holds the service
that allows a client-app to interact with these protocols. Finally
there is an app to use the service and to store files securely on
the skipchain.

## Protocols

Two protocols are used in the onchain-secrets: one to do a Distributed
Key Generation (DKG) using  "Secure Distributed Key Generation for Discrete-Log
Based Cryptosystems" by R. Gennaro, S. Jarecki, H. Krawczyk, and T. Rabin.

DKG enables a group of participants to generate a distributed key
with each participants holding only a share of the key. The key is also
never computed locally but generated distributively whereas the public part
of the key is known by every participants.

The second protocol uses that distributed key to re-encrypt a symmetric
key

## Service

The service ensures the correct usage of the skipchains and offers an
API to the OCS-protocols:

- creating a skipchain
- writing an encrypted symmetric key and a data-blob
- create a read request
- get public key of the Distributed Key Generator (DKG)
- get all read requests

The skipchain has the following transactions:
- write-blocks
	- the symmetrically encrypted document ( <10MB)
	- encryption key (secret-share encrypted)
- read-blocks
	- signed request from a reader for a data-blob
- readers
    - a list of readers that are allowed to access a data-blob. Either a
    static list, or a modifiable list that can be updated by
    one or more administrators

## Conode

A Cothority Node (conode) is a daemon that can run the protocols and
services needed for ocs skipchains. You can run it locally for testing
purposes or install it on a public server to create public cothorities.

## App

ocsmgr is a minimalistic app that interacts with the onchain-secrets skipchain.
For more information, refer to the README-file in <a href="ocsmgr/README.md">ocsmgr</a>

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
