Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Protocols

# Protocols

Describe more:
- onet structure
- roster
- tree
- suites

## List

- [bftcosi](../bftcosi/README.md) is a first
implementation of a Byzantine Fault Tolerant protocol based on PBFT, but using
two rounds of collective signing (cosi) to generate the consensus.
- [byzcoinx](../byzcoinx/README.md) implements
the consensus protocol described in the [OmniLedger paper](https://eprint.iacr.org/2017/406).
- [cosi](../cosi/protocol/README.md) collective
signing, where you can submit a hash of a document and get a collective signature
on it
- [evoting](../evoting/protocol/README.md) uses
different protocols: dkg is a distributed key generated protocol; shuffle is an
implementation of the Neff-shuffle; decrypt is a collective decryption using the
previously run dkg
- [ftcosi](../ftcosi/protocol/README.md) request
and verify collective signatures using part of the bzycoinx protocol
- [messaging](../messaging/README.md) includes
a propagation and a broadcast protocol that is used in multiple services
