Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
Onchain Secrets

# Onchain Secrets

This directory contains a specialized skipchain that implements the
onchain-secrets protocol, developed by DEDIS/EPFL and presented in the
[SCARAB](https://eprint.iacr.org/2018/209.pdf) paper. It allows the storage of
encrypted data on the skipchain with a read-access list and then re-encrypts
that data so that only one of the readers can have access to it. It does so in
a way that the access to the data is always logged, and eventual leakage can be
tracked, or a payment system can be set up. Additionally, the list of readers
can be updated after writing the encrypted data, so that a changing group
of people can access the data. Revocation is also supported.

It uses [Distributed Access Rights Control](darc/README.md) to delegate write
and read rights to groups of people.

## Basic Workflow

This is how onchain-secrets work:

1. Setup
    - An **administrator** asks a **cothority** to set up a skipchain to
    perform a Distributed Key Generation (DKG).
    - The cothority returns the public aggregate key _X_.

2. Writing
    - A **writer** choses a random symmetric key _k_ and uses it to encrypt a
    document _d_ to produce _d_enc_.
    - The writer encrypts the symmetric key using _X_ of the cothority to
    produce _k_enc_.
    - _d_enc_, _k_enc_, and the list of authorized readers are stored on the
    skipchain.

3. Reading
    - A **reader** sends a read request to the cothority and authenticates by
    signing the write-id of the document.
    - The cothority verifies the authentication and adds the read request as a
    new block if the authentication succeeds.
    - The reader requests a re-encryption of _k_enc_ under the reader's public
    key and receives _k_enc_reader_.
    - Using his/her private key, the reader can recover _k_. From the skipchain
    he/she can get _d_enc_ and recover the original document _d_.

4. Auditing
    - An **auditor** can traverse the skipchain and see when a reader accessed
    a certain document.

## Links

- [OCS Command Line Interface](CLI.md)
- [OCS Reencryption Protocol](protocol/Reencrypt.md)
- [OCS Distributed Key Generation](protocol/DKG.md)
- [Client API](service/README.md) offers an API to connect from a client to an
OCS service
- [Distributed Access Rights Control](darc/README.md) - the data structure used
to define access control
- [SCARAB](https://eprint.iacr.org/2018/209.pdf) - Hidden in Plain Sight
- [Skipchain](../skipchain/README.md) is the storage data structure used for the
transactions
