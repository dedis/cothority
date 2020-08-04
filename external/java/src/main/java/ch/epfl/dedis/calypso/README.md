# Calypso

Some notes on how to use Calypso in ByzCoin. Calypso allows you to do the following:

Store encrypted data together with a decryption key on ByzCoin, in a way that the decryption key is only accessible
by trusted parties and only after they authenticated to ByzCoin and proved that they have access.

Given a document with a text `secret` that needs to be put on ByzCoin, the following steps are needed:

1. create a symmetric key and encrypt the `secret` using this symmetric key
2. encrypt the symmetric using the Long Term Secret public key
3. send the encrypted `secret` and the encrypted symmetric key to ByzCoin, together with a list of access control rules

Now the document is stored on ByzCoin and is ready to be read by anybody being in the list of access control rules.
Supposing the reader has the `InstanceID` of the `WriteInstance`, he can do the following:

1. get the encrypted `secret` from the `WriteInstance`
2. send a read-request to the `WriteInstance` in ByzCoin and wait for a proof that the request is stored
3. send the write-proof and read-proof to the Long Term Secret, getting the symmetric key
4. decrypt the symmetric key, and use it to decrypt the `secret` text

## Setup

First of all a new `CalypsoRPC` has to be created, to set up the _Long Term Secret_ that will encrypt and decrypt the
symmetric key.

## Write, Read, and DecryptKey

Three basic structures are used to work with Calypso:

1. Write
   - `WriteData` - holds the encrypted `secret` and the encrypted symmetric key, as well as an optional plain text piece
  of data
   - `WriteInstance` - once the WriteData is stored in ByzCoin, it is stored in the form of a WriteInstance. The corresponding
  java class has methods to interact with ByzCoin and Calypso.
2. Read
   - `ReadData` - holds a reference to the `WriteInstance` and who is allowed to request the symmetric key
   - `ReadInstance` - once the `ReadData` is stored in Byzcoin, it is stored in the form of a `ReadInstance`. The corresponding
   java class has methods to interact with ByzCoin and Calypso.
3. `DecryptKey`
   - Once a `ReadInstance` has been created in ByzCoin, it can be used to get the symmetric key by sending a proof of
   the `WriteInstance` and the `ReadInstance` to Calypso, which will verify the proofs, and if successful, send back the
   decrypted symmetric key (in fact it re-encrypts it to the reader).

## Document and Encryption

As an example encryption two java classes, `Document` and `Encryption`, are written to give a simple
example of how to encrypt the data before storing it in ByzCoin.
