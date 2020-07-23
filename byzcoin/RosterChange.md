Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/doc/BuildingBlocks.md) ::
[ByzCoin](https://github.com/dedis/cothority/tree/master/byzcoin/README.md) ::
RosterChange

# Roster Change

As ByzCoin is a permissioned blockchain, we know the roster that we'll use for
the consensus algorithm. The first element of the roster is the leader who is
proposing new blocks to the followers.

The current implementation only allows adding or removing one node at a time,
mostly to avoid making the system unstable before changing the roster.

In case of a viewchange, we can simply rotate that roster and define the new view
with the new leader at the beginning of the roster. It is more difficult, however,
to add a new node to the roster, as the new node will have to get the new state
before being able to participate in the consensus.

There are two cases for the new node:
1. it already has a version of the global state that is not older than
a given threshold of blocks
2. it doesn't have a version, or it is too old to be considered

## Downloading new blocks

When there is a snapshot of the global state and it is not too old, then the new
node asks an existing node for all the missing blocks. The new node can then
create the new global state by applying all transactions from the blocks.

It might happen that this update takes too long and that new blocks arrive. If
this is the case, the procedure repeats and the node should be able to catch
up sooner or later with the rest of the nodes.

## Download the global state

If there are too many blocks missing the node can decide it's better to download
the data of the global state. This data can be multiple gigabytes, so it can take
quite some time to download that data.

The threshold of missing blocks to download the global state must take into account
the time it takes to download the global state, else the node will be constantly
downloading the global state, only to find himself out of date once the download
is complete.
