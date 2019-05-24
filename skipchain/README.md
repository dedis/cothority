Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
Skipchain

# Skipchain implementation

> Skipchains are authenticated data structures that combine ideas from 
> blockchains and skiplists. Skipchains enable clients
> to securely traverse the timeline in both forward and backward directions 
> and to efficiently traverse short or long distances by employing 
> multi-hop links. Backward links are cryptographic hashes of past blocks, 
> as in regular blockchains. Forward links are cryptographic signatures of 
> future blocks, which are added retroactively when the target block appears 
> [1](https://www.usenix.org/system/files/conference/usenixsecurity17/sec17-nikitin.pdf).

This skipchain implementation is based upon the Chainiac-paper
[1](https://www.usenix.org/system/files/conference/usenixsecurity17/sec17-nikitin.pdf)
and offers a very simple and extendable voting-based permissioned blockchain.
Instead of having one global blockchain, the skipchain implementation allows
users to use a personal blockchain where they can store data. Skipchain is used
by - but not limited to - the following elements of the cothority:

- [byzcoin](../byzcoin) - a scalable Byzantine fault tolerant consensus
algorithm for open decentralized blockchain systems. It uses the skipchain as its
underlying data-structure for storing blocks.
- [caypso](../calypso) - a fully decentralized framework for auditable access control
on protected resources.
- [eventlog](../eventlog) - an auditable and secure logging service for byzcoin
- [RandHound](https://github.com/dedis/paper_17_randomness) - a service that produce 
verifiable random numbers and store them on a blockchain for later proofs

## Overview of implementation

The skipchain itself is held by one or more conodes. Clients can connect to the
leader and propose new blocks. Every time a new block is received, the leader
runs a BFT-protocol with the other conodes. All conodes keep a copy of the
skipchain-blocks.

## Usage

A simple first step on how to use skipchains is described in the
skipchain-manager readme: [SCMGR](../scmgr/README.md).

Another usage example is in [CISC](../cisc/README.md).

# Catch-up Behavior

If the conode is a follower for a given skipchain, then when it is asked to add
a block onto that skipchain by the leader, it will contact other conodes in the
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
