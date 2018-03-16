Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
Onchain Secrets

# Onchain Secrets

This is a first implementation of a skipchain that implements the
onchain-secrets protocol as developed by DEDIS/EPFL and presented in the
[SCARAB](https://eprint.iacr.org/2018/209.pdf) paper. It allows the
storage of encrypted data on the skipchain with a read-access list
and then re-encrypts that data so that only one of the readers can
have access to it. In such a way the access to the data is always
logged, and eventual leakage can be tracked, or a payment system can
be set up. Additionally, the list of readers can be updated after the
write of the encrypted data, so that changing groups of people can access
the data, or that access can be revoked.

It also uses [Distributed Access Rights Control](darc/README.md) to delegate
write and read rights to groups of people.

## Basic Workflow

This is how onchain-secrets work:

1. Setup
    - an **administrator** asks a **cothority** to set up a skipchain
    and to perform a Distributed Key Generation (DKG)
    - the cothority returns the public aggregate key _X_

2. Writing
    - a **writer** choses a random symmetric key _k_ and uses it to encrypt
    a document _d_ to produce _d_enc_
    - the writer encrypts the symmetric key using _X_ of the cothority
    to produce _k_enc_
    - _d_enc_, _k_enc_, and the list of authorized readers are stored on the
    skipchain

3. Reading
    - a **reader** sends a read request to the cothority and authenticates
    by signing the write-id of the document
    - the cothority verifies the authentication and adds the read request
    as a new block if the authentication succeeds
    - the reader requests a re-encryption of _k_enc_ under the reader's
    public key and receives _k_enc_reader_
    - using his private key, the reader can recover _k_. From the skipchain
    he can get _d_enc_ and recover the original document _d_

4. Auditing
    - an **auditor** can traverse the skipchain and see when a reader
    accessed a certain document.

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
