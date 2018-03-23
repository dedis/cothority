Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
Skipchain

# Skipchain implementation

This skipchain implementation is based upon the Chainiac-paper
(https://www.usenix.org/system/files/conference/usenixsecurity17/sec17-nikitin.pdf)
and offers a very simple and extendable voting-based permissioned blockchain.
Instead of having one global blockchain, the skipchain implementation allows
users to use a personal blockchain where they can store data. In the cothority,
the following data types are implemented:

- [CISC](../cisc/README.md) - offers a key/value storage where
a threshold of devices need to sign off on changes. Currently supported
key/values are:
  - SSH-keys - storing public ssh-keys that will be followed by a ssh-server for
  passwordless logins
  - Web-pages - store your webpage on your personal blockchain and only allow
  changes if a threshold of administrators sign off
  - Free values - add any key/value pair to your personal skipchain
- [RandHound](../randhound/README.md) - produce verifiable random numbers
and store them on a blockchain for later proofs
- [OCS](../ocs/README.md) - a distributed, secure document sharing service that
features access-control and auditability. The documents and access-control
lists are stored on the skipchain.

## Overview of implementation

The skipchain itself is held by one or more conodes. Clients can connect to the
leader and propose new blocks. Every time a new block is received, the leader
runs a BFT-protocol with the other nodes. All nodes keep a copy of the
skipchain-blocks.

## Usage

A simple first step on how to use skipchains is described in the
skipchain-manager readme: [SCMGR](../scmgr/README.md).

Another usage example is in [CISC](../cisc/README.md).

# Catch-up Behavior

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
