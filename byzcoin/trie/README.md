Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/main/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/main/doc/BuildingBlocks.md) ::
[ByzCoin](https://github.com/dedis/cothority/blob/main/byzcoin/README.md) ::
Trie

Trie: An implementation of the Merkle prefix tree from CONIKS
=============================================================

This package contains the implementatin of the Merkle Prefix Tree from
[CONIKS](https://www.usenix.org/system/files/conference/usenixsecurity15/sec15-paper-melara.pdf).
The implementation honours the paper with the exception that the values in the
leaf nodes are not commitments but are regular key/value pairs. Nevertheless,
the values are simply byte slices, so it's easy to make a wrapper API that
stores commitments as values.

We support two types of storage backends: in-memory and on-disk (via
[boltdb](https://github.com/etcd-io/bbolt)). The in-memory version is good for
testing or used as a temporary because the data does not persist upon closing.
Nevertheless, it is possible to copy from one backend to another.

Trie
----
`Trie` is the main data structure of the package. The functions that you'll use
the most often are `Set`, `Get` and `Delete`. The value is committed if the
operation returns without an error. Additionally, we support batch processing
using `Batch`, where the input is a set of operations and these will be
processed atomically.

To proof whether a value exists, `GetProof` should be used. It will return a
hash-chain from the root to either the leaf node, which contains the value, or
an empty node, proving the existence or absence.


Staging Trie
------------
A `StagingTrie` can be created from a source `Trie`. It has a similar API
except that the operations are not committed to the source `Trie` until
`Commit` is called. Under the hood, `StagingTrie` keeps un-committed operations
in memory. If the root or the proof needs to be computed, it will apply these
operations to the source `Trie`, compute the result (e.g., the root) and then
revert the changes from the source. So the staging trie should not hold too
many un-committed operations otherwise the `GetProof` and `GetRoot` functions
will slow down significantly.
