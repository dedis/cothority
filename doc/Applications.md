Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Applications

# Applications

An application is a higher level of abstraction and fulfils a purpose that is
bigger than that of a library. It is not used by any other building block or
application, but rather uses building blocks itself.

It is difficult to draw a clear line between libraries and applications in the
cothority repository. This is because even the libraries can have a command
line interface to interact between a client and the library. So the presence
of a CLI is not enough to be an application, while it is a requirement.

The currently elected applications in the cothority are:
- [Status Report](../status/README.md) reports the status of a node
- [Calypso](../calypso/README.md) hides data on a blockchain and adds
an access control to it
- [E-voting](../evoting/README.md) run an election by storing votes on a blockchain,
then having a cothority shuffling them and decrypting the votes.
- [Eventlog](../eventlog/README.md) is an event logging system built on top of ByzCoin.

# Building Blocks

These two pieces of technology support those above:

- [ByzCoin](../byzcoin/README.md) has a distributed ledger holding keys and
values. It implements pre-compiled smart contracts.
- [Skipchain](../skipchain/README.md) is a blockchain maintained by a cothority
with features that allow clients to catch up without downloading the entire chain.
