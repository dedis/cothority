Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Services

# Services

Describe in more detail:
- protobuf over websockets
- internal Communication
- storage of data

## List

Here is a list of all services available in the cothority:

- [cosi](../cosi/service/README.md) collective
signing, where you can submit a hash of a document and get a collective signature
on it
- [identity](../identity/README.md) a
distributed key/value storage handled by a skipchain and with applied verification
functions.
- [evoting](../evoting/service/README.md) run
an election on a decentralized system using skipchains to store the votes
- [ftcosi](../ftcosi/service/README.md) request and verify
collective signatures using part of the bzycoinx protocol
- [pop](../pop/service/README.md) create and participate
in Proof-of-Personhood parties where each participant gets a cryptographic token
that identifies him anonymously as a unique person
- [skipchain](../skipchain/README.md) a permissioned
blockchain for storing arbitrary data if a consensus of a group of nodes is found
- [status](../status/service/README.md) returns the status of a conode
