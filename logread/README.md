# Write log-read skipchain

This is a first implementation of a skipchain that has the following features:

- writing a secret threshold-encrypted to the skipchain
- asking for read-permission and writing the permission to the skipchain

It uses two kind of skipchains:

- acl-skipchain with the following rights:
	- admin: a threshold can update the skipchain
	- read: any public key in here can ask for read access
	- write: these keys here can add new documents to the skipchain
- document-skipchain with a structure that holds:
	- link to the encrypted document
	- secret-shared encrypted master password
	- link to acl-skipchain