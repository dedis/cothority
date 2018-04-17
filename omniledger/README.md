Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
Omniledger

# Omniledger implementation

This implementation of Omniledger has its goal to implement the protocol
described in the [Omniledger Paper](https://eprint.iacr.org/2017/406.pdf).
As the paper is only describing the network interaction and very few of the
details of how the transactions themselves are handled, we define the
following details:

- multiple transactions per block
- key/value storage is protected by verification functions
- queueing of transactions at the nodes and collection by the leader
- view-change of the leader if he stalls
