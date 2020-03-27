PatriciaTrie: An implementation of the Ethereum Merkle Patricia Trie
====================================================================

This package implements the [merkle patricia trie](https://github.com/ethereum/wiki/wiki/Patricia-Tree#modified-merkle-patricia-trie-specification-also-merkle-patricia-tree) used in Ethereum and Hyperledger Indy
to store the state.

However, unlike Ethereum which uses Keccack-256 hashing algorithm for determining the `key` in
the persistent storage layer, this package uses the SHA3-256 algorithm which is used by
Hyperledger Indy.

Like the [trie](../../byzcoin/trie/README.md) package, this implementation supports in-memory and
on-disk backends using the same API.


PatriciaTrie
------------

This data structure allows users to store key-value pairs in the trie using the
`Get` and `Put` methods.

The changes to the trie are not persisted to the storage backend until the
user calls the `Commit` method on the trie instance.

The root hash for the trie may be retrieved by calling the `RootHash` method.
The value of a particular key as it existed for a given root hash may be obtained
by calling the `GetAtRoot` method.
