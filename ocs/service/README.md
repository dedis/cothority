Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../../README.md) ::
[Applications](../../doc/Applications.md) ::
[Onchain Secrets](../README.md) ::
Client API

# Client API

A service interacts with the outer world through an API that defines the
methods that are callable from the outside. The service can hold data that
will survive a restart of the cothority and instantiate any number of
protocols before returning a value.

Inside of the OCS, extensive use of [Distributed Access Rights Control](../darc/README.md)
is made to delegate rights to write new documents and to read the written documents.

The OCS service uses skipchain to store the transactions on distributed nodes.
In this proof of concept implementation, each transaction is stored in its own
block. Only the new Omniledger implementation will allow to collect multiple
transactions into one block.

A transaction is a protobuf message with the following fields:
- Write
	- the symmetrically encrypted data ( <10MB)
	- encryption key (secret-share encrypted)
- Read
	- signed request from a reader for a data-blob
- Readers
    - a list of readers that are allowed to access a data-blob. Either a
    static list, or a modifiable list that can be updated by
    one or more administrators
- Metadata
  - Can represent any data the client wants to store in the SkipBlock
- Timestamp
  - Is verified by the conodes to be within 1 minute of their clock

## API Overview

The ocs-service implements the following methods:

- creating an OCS-skipchain
- writing an encrypted symmetric key and a data-blob
- create a read request
- get public key of the Distributed Key Generator (DKG)
- get all read requests

All messages are sent as protobuf over websockets. We have an implementation
for go programs that can connect to a conode to use the OCS service, and another
implementation in Java to connect to a conode.

### CreateSkipchain

CreateSkipchain creates a new OCS-skipchain using the roster r. The OCS-service
will create a new skipchain with an empty first genesis-block. You can create more
than one skipchain at the same time.

Input:
```
- r [*onet.Roster] - the roster of the nodes holding the new skipchain
- admin [*darc.Darc] - the administrator of the ocs-skipchain
```

Returns:
```
- ocs [*SkipChainURL] - the identity of that new skipchain
- err - an error if something went wrong, or nil
```

### EditAccount

EditAccount creates a new account on the skipchain. If the account-ID already exists,
there must be a valid signature provided in the Darc-structure, and all elements
must be valid: Version_new = Version_old + 1, Threshold_new = Threshold_old and the
different Darc-changes must follow the rules.

### WriteRequest

WriteRequest contacts the ocs-service and requests the addition of a new write-
block with the given encData. The encData has already to be encrypted using the symmetric
symKey. This method will encrypt the symKey using the public shared key of the
ocs-service and only send this encrypted key over the network. The block will also
contain the list of readers that are allowed to request the key.

Input:
```
- ocs [*SkipChainURL] - the url of the skipchain to use
- encData [[]byte] - the data - already encrypted using symKey
- symKey [[]byte] - the symmetric key - it will be encrypted using the shared public key
- adminKey [kyber.Scalar] - the private key of an admin
- acl [Darc] - the access control list of public keys that are allowed to access
  that resource
```

Output:
```
- sb [*skipchain.SkipBlock] - the actual block written in the skipchain. The
  Data-field of the block contains the actual write request.
- err - an error if something went wrong, or nil
```

### ReadRequest

ReadRequest is used to request a re-encryption of the symmetric key of the
given data. The ocs-skipchain will verify if the signature corresponds to
one of the public keys given in the write-request, and only if this is valid,
it will add the block to the skipchain.

Input:
```
- ocs [*SkipChainURL] - the url of the skipchain to use
- data [skipchain.SkipBlockID] - the hash of the write-request where the
  data is stored
- reader [kyber.Scalar] - the private key of the reader. It is used to
  sign the request to authenticate to the skipchain.
```

Output:
```
- sb [*skipchain.SkipBlock] - the read-request that has been added to the
  skipchain if it accepted the signature.
- err - an error if something went wrong, or nil
```

### DecryptKeyRequest

DecryptKeyRequest takes the id of a successful read-request and asks the cothority
to re-encrypt the symmetric key under the reader's public key. The cothority
does a distributed re-encryption, so that the actual symmetric key is never revealed
to any of the nodes.

Input:
```
- ocs [*SkipChainURL] - the url of the skipchain to use
- readID [skipchain.SkipBlockID] - the ID of the successful read-request
- reader [kyber.Scalar] - the private key of the reader. It will be used to
  decrypt the symmetric key.
```

Output:
```
- sym [[]byte] - the decrypted symmetric key
- err - an error if something went wrong, or nil
```

### DecryptKeyRequestEphemeral

DecryptKeyRequestEphemeral works similar to DecryptKeyRequest but generates
an ephemeral keypair that is used in the decryption. It still needs the
reader to be able to sign the ephemeral keypair, to make sure that the read-
request is valid.

Input:
```
- ocs [*SkipChainURL] - the url of the skipchain to use
- readID [skipchain.SkipBlockID] - the ID of the successful read-request
- reader [*darc.Signer] - the reader that has requested the read
```

Output:
```
- sym [[]byte] - the decrypted symmetric key
- cerr [ClientError] - an eventual error if something went wrong, or nil
```

### GetReadRequests

GetReadRequests searches the skipchain starting at 'start' for requests and returns all found
requests. A maximum of 'count' requests are returned. If 'count' == 0, 'start'
must point to a write-block, and all read-requests for that write-block will
be returned.

Input:
```
- ocs [*SkipChainURL] - the url of the skipchain to use
```

Output:
```
- err - an error if something went wrong, or nil
```

### GetLatestDarc

GetLatestDarc looks for an update path to the latest valid
darc given either a genesis-darc and nil, or a later darc
and its base-darc.
