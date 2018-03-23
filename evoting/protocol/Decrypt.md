Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../../README.md) ::
[Building Blocks](../../doc/BuildingBlocks.md) ::
Distributed Decryption

# Distributed Decryption

Once a [Distributed Key](DKG.md) has been setup, the aggregate public key can
be used to encrypt a document. Such a document cannot be decrypted by any
party alone, but needs the collaboration of a threshold of nodes to decrypt
it.

The _decrypt_ protocol in this directory asks every node to participate in
the decryption of an encrypted data blob. In the current state it is very
strongly tied to the evoting service and verifies that the current state of
the vote allows to decrypt.
