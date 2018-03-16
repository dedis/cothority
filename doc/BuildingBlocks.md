Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Building Blocks

# Building Blocks

Building blocks are grouped in terms of research units and don't have a special
representation in code. Also, the _General Crypto_ library is more of a
collection of different protocols.

## Consensus

Whenever you have a distributed system the question of consensus arises: how do
you make sure that a majority of nodes see the same state? At DEDIS we're using
collective signatures as a base to build a PBFT-like consensus protocol.
Based on that we implemented a skipchain that can store arbitrary data in blocks
and use a verification function to make sure all nodes agree on every block.

- [Collective Signing](../cosi/README.md)
is the basic signing algorithm we've been using - replaced by:
- [Fault Tolerant Collective Signing](../ftcosi/README.md)
a more fault tolerant version of the CoSi protocol with only a 3-level tree
- [Byzantine Fault Tolerant CoSi](../bftcosi/README.md)
is an implementation inspired by PBFT using two rounds of CoSi
- [ByzCoinX](../byzcoinx/README.md)
the implementation of the basic consensus protocol in the Omniledger paper

## Key Sharding

Another useful tool for distributed system is key sharing, or key sharding.
Instead of having one key that can be compromised easily, we create an
aggregate key where a threshold number of nodes need to participate to
decrypt or sign. Our blocks can do a publicly verifiable distributed
key generation as well as use that sharded key to decrypt or reencrypt data
to a new key without ever having the data available.

- [Distributed Key Generation](../evoting/DKG.md)
uses the protocol presented by Rabin to create a distributed key
- [Distributed Decryption](../evoting/Decrypt.md)
takes an ElGamal encrypted ciphertext and decrypts it using nodes
- [Re-encryption](../ocs/protocol/Reencrypt.md)
re-encrypts an ElGamal encryption to a new key while never revealing the original
data

## General Crypto

This is our _one size fits all_ collection of blocks that are useful in different
places, but not tied to one specific application.

- [Neff](../evoting/protocol/Neff.md)
- [RandHound](../randhound/README.md)

## Messaging

Finally some building blocks useful in most of the services.

- [Broadcast and Propagation](../messaging/README.md)
