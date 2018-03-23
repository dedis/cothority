Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
CISC

# CISC - Cisc Identity SkipChain

Cisc uses a personal blockchain handled by the cothority. It
can store key/value pairs, and has special modules for managing
ssh-public-keys and web-pages.

Based upon skipchains, cisc serves a data-block with different entries that can
be handled by a number of devices who propose changes and cryptographically vote
to approve or deny those changes. Different data-types exist that will interpret
the data-block and offer a service.

Besides having devices that can vote on changes, simple followers can download
the data-block and get cryptographically signed updates to that data-block to be
sure of the authenticity of the new data-block.

## Terms

Here is an overview of the terms used in cisc:
- CISC - Cisc Identity SkipChain
- Skipchain - blockchain structure developed by the EPFL/DEDIS lab
- Conode - a server program offering services like cisc and others
- Device - a computer that has voting power on an identity-skipchain
- Data - all key/value pairs stored on the SkipChain
- Proposed Data - data that has been proposed but not yet voted with a threshold

## Block Content

Each block in a CISC skipchain contains the following information:

- Threshold of how many devices need to sign to accept the new block
- A list of all devices allowed to sign
- All key/values stored in CISC (yes, this is non-optimal :)
- the new proposed roster - nil if the old is to be used
- Votes for that block

## Using cisc to handle your ssh-keys

Please be aware that this is still quite experimental - so always back up your
ssh private keys and make sure you have an alternative way of logging in to
your server who is following the keys.

1. Create your own, personal blockchain, using the DEDIS-Cothority or your own
2. Add devices who are allowed to evolve the blockchain
3. Add ssh-keys to the blockchain
4. Follow the blockchain with a server


## Links

- [CISC Command Line Interface](CLI.md)
- [CISC CLI Reference](Reference.md)
- [Client API](../identity/README.md)
- [Skipchain service](../skipchain/README.md)
