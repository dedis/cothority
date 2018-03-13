Navigation: [DEDIS](https://github.com/dedis/doc/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
Skipchain

# Skipchain implementation

This skipchain implementation is based upon the Chainiac-paper (*LINK*) and offers
a very simple, extendable voting-based permissioned blockchain. Instead of having
one global blockchain, the skipchain implementation allows users to use a personal
blockchain where they can store data. In the cothority, the following data
types are implemented:

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

## Overview of implementation

The skipchain itself is held by one or more conodes. Clients can connect to the leader and propose new blocks. Every time a new
block is received, the leader runs a BFT-protocol with the other
nodes. All nodes keep a copy of the skipchain-blocks.

## Usage

A simple first step on how to use skipchains is described in the skipchain-manager
readme: [SCMGR](../scmgr/README.md)

Another usage example is in [CISC](../cisc/README.md).
