# OCS-Service

A service interacts with the outer world through an API that defines the
methods that are callable from the outside. The service can hold data that
will survive a restart of the cothority and instantiate any number of
protocols before returning a value.

The ocs-service implements the following methods:

- creating a skipchain
- writing an encrypted symmetric key and a data-blob
- create a read request
- get public key of the Distributed Key Generator (DKG)
- get all read requests
