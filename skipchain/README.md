# Skipchain implementation

This skipchain implementation is based upon the Chainiac-paper (*LINK*) and offers
a very simple, extendable voting-based permissioned blockchain. Instead of having
one global blockchain, the skipchain implementation allows users to use a personal
blockchain where they can store data. In the cothority, the following data
types are implemented:

* CISC [github.com/dedis/cothority/cisc] - offers a key/value storage where
a threshold of devices need to sign off on changes. Currently supported
key/values are:
  * SSH-keys - storing public ssh-keys that will be followed by a ssh-server for
  passwordless logins
  * Web-pages - store your webpage on your personal blockchain and only allow
  changes if a threshold of administrators sign off
  * Free values - add any key/value pair to your personal skipchain
* RandHound [github.com/dedis/cothority/randhound] - produce verifiable random numbers
and store them on a blockchain for later proofs

## Overview of implementation

The skipchain itself is held by one or more conodes. Clients can connect to the leader and propose new blocks. Every time a new
block is received, the leader runs a BFT-protocol with the other
nodes. All nodes keep a copy of the skipchain-blocks.

## Usage

A simple first step on how to use skipchains is described in the skipchain-manager
readme: [github.com/dedis/cothority/scmgr/README.md].

Another usage example is in CISC: [github.com/dedis/cothority/cisc/README.md]
