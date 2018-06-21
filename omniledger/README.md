Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/doc/BuildingBlocks.md) ::
OmniLedger

# OmniLedger

This implementation of OmniLedger has its goal to implement the protocol
described in the [OmniLedger Paper](https://eprint.iacr.org/2017/406.pdf).
As the paper is only describing the network interaction and very few of the
details of how the transactions themselves are handled, we will include
them as seem fit.

This document describes the part of omniledger that are implemented and how to
use them. It should grow over time as more parts of the system are implemented.

## Overview

Here is a graphical overview of the current implementation in the cothority
framework:

![Omniledger Implementation](Omniledger.png?raw=true "Omniledger")
As an svg: [Omniledger Implementation](Omniledger.svg). This image has been
created with https://draw.io and can be imported there.

Our omniledger service currently implements:

1. multiple transactions per block
2. queuing of transactions at each node and periodical creation of a new
block by the leader
3. contracts that define the behaviour of how to change the global state

The following points are scheduled to be done before end of '18:

4. view-change in case the leader fails
5. sharding of the nodes
6. inter-shard transactions

1-4 are issues that should've been included in skipchains for a long time, but
never got in. Only 5-6 are 'real' omniledger improvements as described in the
[OmniLedger Paper](https://eprint.iacr.org/2017/406.pdf).

The current implementation is doing 1-3.

# Structure Definitions

Following is an overview of the most important structures defined in OmniLedger.
For a more programmatic description of these structures, go to the
[DataStructures](DataStructures.md) file.

## Skipchain Block

Whenever OmniLedger stores a new Skipchain Block, the header will only contain
hashes, while the clientTransactions will be stored in the body. This allows
for a reduced proof size.

Block header:
- Merkle tree root of the global state
- Hash of all clientTransactions in this block
- Hash of all stateChanges resulting from the clientTransactions

Block body:
- List of all clientTransactions

## Smart Contracts in OmniLedger

Previous name was _Precompiled Smart Contracts_, but looking at how we want
it to work, we decided to call it simply a set of Contracts. A contract defines
how to interpret the methods sent by the client. It is identified by the
contractID which is a string pointing to a given contract.

Contracts receive as an input a list of coins that are available to them. As
an output, a contract needs to give the new list of coins that is available.

After all contracts have been run, the leftover coins are given to the leader as
a mining reward.

Input arguments:
- pointer to database for read-access
- Instruction from the client
- key/value pairs of coins available

Output arguments:
- one StateChange (might be empty)
- updated key/value pairs of coins still available
- error that will abort the clientTransaction if it is non-zero. No global
state will be changed if any of the contracts returns non-zero.

## From Client to the Collection

In OmniLedger we define the following path from client instructions to
global state changes:

* _Instruction_ is one of Spawn, Invoke or Delete that is called upon an
existing object
* _ClientTransaction_ is a set of instructions sent by a client
* _StateChange_ is calculated at the leader and verified by every node. It
contains the new key/contractID/value triplets to create/update/delete.

A block in omniledger contains zero or more OmniLedgerTransactions. Every
one of these transactions can be valid or not and will be marked as such by
the leader. Every node has to verify whether it accepts or refuses the
decisions made by the leader.

### Authentication and Coins

Current authentications support darc-signatures, later authentications will also
support use of coins. It is the contracts' responsibility to verify that enough
coins are available.

## Collection

The collection is a Merkle-tree based data structure to securely and
verifiably store key / value associations on untrusted nodes. The library
in this package focuses on ease of use and flexibility, allowing to easily
develop applications ranging from simple client-server storage to fully
distributed and decentralized ledgers with minimal bootstrapping time.

Our collection used is a library that has been
[developed for a PhD project](collection/README.md) and
can do much more than simple Merkle-trees. Depending on the future direction
of the project, it might be replaced by a simpler Merkle-tree implementation.

## Darc

Package darc in most of our projects we need some kind of access control to
protect resources. Instead of having a simple password or public key for
authentication, we want to have access control that can be: evolved with a
threshold number of keys be delegated. So instead of having a fixed list of
identities that are allowed to access a resource, the goal is to have an
evolving description of who is allowed or not to access a certain resource.

## Further reading

Some documents that might get evolved later:

- [Child Transactions](ChildTransactions.md) describes how we can implement
a leader fetching new transactions from children.
- [Contracts](Contracts.md) gives a short overview how contracts work and
some examples how to use them.
