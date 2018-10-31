Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/doc/BuildingBlocks.md) ::
[ByzCoin](README.md) ::
Instance Versioning

# Instance Versioning

This document gives a detailed overview of the instance versioning that allows
security check and history navigation.

## Versioning

Each state change has a `Version` field populated with the version of the instance
after having applied the change. It is monotonically increasing by one every time
an instruction is performed on an instance.
The global state also stores the version at the moment of the block creation because
we need to know what is the latest version to create new transaction asking the right
version. This prevents replay attacks (see #1442).
The Byzcoin service takes care of the versioning by modifying the results of a
contract execution. It will get the current version in the global state and then
increase the value for each instruction.

### Per state change VS per client transaction

It has been decided to go for a version per state change because we are interested
in the state change history, the client transaction one which means that we need
a more fine grain approach where each instruction will create a new state change
and thus a new version. Using a version per client transaction would have yeld
state changes without version or several with the same one.

## Storage

Each service stores the state changes after a new block has been added and only
at this moment meaning that until a pending transaction is waiting to be included
in a block, it will not be part of the instance's history.
The storage uses BoltDB and the key is generated using the instance ID and the
version number so that the pairs are sorted first by instance ID and then by
version.

## Backup and new conode

This storage acts more like a cache and it could happen a conode needs to create it
from scratch to catch up with the others if we change the roster or if the DB fails,
for instance.
When a conode starts a Byzcoin service, it will first try to synchronize all the
skipchains created in that roster. After this step is successful, the service will
start a routine to populate the cache by reading the skipchain from the beginning
or one of the snapshot of the global state (not yet available #????). Note that
the history will be as old as the snapshot.

## Size management

This feature needs a size control as the space taken will grow with the skipchain and
that's why you can specify the size of the cache or the maximum number of blocks that
need to be kept. When a size is specified, the storage will look for the oldest 
elements and delete them until 20% of the space is available. Note that if state
changes has been added unsorted, it will remove the oldest version of the instance
that contains the oldest element to prevent holes. When a maximum number of blocks
is specified, it will keep N blocks for each instance and remove the others.