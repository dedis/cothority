# Write log-read skipchain

This is a first implementation of a skipchain that has the following features:

- writing a secret threshold-encrypted to the skipchain
- asking for read-permission and writing the permission to the skipchain

It uses two kind of skipchains:

- acl-skipchain with the following rights:
	- admin: a threshold (for the moment 1) can update the skipchain
	- write: these keys can add new documents to the skipchain
	- read: any public key in here can ask for read access
	
- document-skipchain with a structure that holds:
	- configuration-blocks
		- link to acl-skipchain
	- write-blocks
		- the symmetrically encrypted document ( <10MB)
		- encryption key (will be secret-share encrypted)
	- read-blocks
		- signed request from a reader for a file

To handle the on-chain secrets, two additional methods are available that still
need to be implemented correctly:

- EncryptKey: returns a public key with which the writer can encrypt his
encryption key
- DecryptKey: returns the encryption key for the file asymmetrically encrypted using the
reader's public key

## App

There is a simple app that can be used to set up and interact with the skipchain.
For more information, refer to the README-file in <a href="wlogr/README.md">wlogr</a>

## Service

The service ensures the correct usage of the skipchains:
- the ACL-skipchain only evolves when a new ACL signed by a previous admin is
proposed
- the WLR-skipchain allows only write-requests from writers in the ACL-skipchain
and read-requests from readers

TODO:

- Implement the EncryptKey and DecryptKey methods to have protected keys.

# Docker-files

A handy docker-file exists for easy testing of the logread-skipchain. You
can build it using:

```bash
make docker
```

Once it's built, run it with

```bash
make docker_run
```

## Testing the docker-file

If the docker-file is running, you can create a simple chain, add a document
and read it again:

```bash
go run example/main.go
```
