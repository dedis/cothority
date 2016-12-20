[![Build Status](https://travis-ci.org/dedis/cothority.svg?branch=master)](https://travis-ci.org/dedis/cothority)
[![Coverage Status](https://coveralls.io/repos/github/dedis/cothority/badge.svg)](https://coveralls.io/github/dedis/cothority)

# Cothority-ONet

* **Cothority** - Collective Authority
* **ONet** - Overlay network

The ONet project offers a framework for research, simulation and deployment 
of crypto-related protocols with an emphasis of decentralized, distributed protocols.
The cothority-repository holds protocols, services and apps for you to use.
You can find more information at
https://github.com/dedis/cothority/wiki

## Apps

So you want to use one of our services and you are interested in one of our projects:

* [Cothority](https://github.com/dedis/cothority/wiki/Cothority) - The main server being able to handle all service-requests
* [CoSi] - Collective Signing, where you can submit a hash of a document and get a collective signature on it
* [Cisc] - Cisc Identity Skipchain, a distributed key/value storage handled by a permissioned, personal blockchain with an SSH-plugin

## Services

Some of the services don't have yet an application, but can be interesting anyway:

* [[ServiceTimestamper]]

## Protocols

This is a short overview of the available protocols in the cothority-repository:

### Byzcoin 

XXX Need references.
ByzCoin implements the ByzCoin protocols which consists of two CoSi Protocols:
* One Prepare phase where the block is dispatched and signed amongst the tree.
* One Commit phase where the previous signature is seen by all the participants
  and they make a final one out of it.
This implicitly implements a Byzantine Fault Tolerant protocol.

### PBFT

PBFT is the Practical Fault Tolerant algorithm described
in http://pmg.csail.mit.edu/papers/osdi99.pdf .
It's a very simple implementation just to compare the performance relative to
ByzCoin.

### Ntree

Ntree is like ByzCoin but with independant signatures. There is no Collective
Signature (CoSi) done. It has been made to compare the perform relative to
ByzCoin.

## More documentation

Our documentation is split in three parts: 

* **Users**, when all you want is to use one of our services - read further down. [Cothority](https://github.com/dedis/cothority/wiki)
* **PhD**, for those of you who have an idea and want to implement it using Onet, you can go to 
[Cothority_Template](https://github.com/dedis/cothority_template/wiki)
* **Developer**, hard-core hackers that want to make our code even better and faster. You can go to [Onet](https://github.com/dedis/onet/wiki)

## Development

If you want to help with development, you can start picking one of the
entry-level issues at https://github.com/dedis/cothority/issues/524
