Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
Onchain-Secrets

# Onchain-Secrets (OCS)

Calypso is a system to store secrets in plain sight, for example in a blockchain.
It is composed in two parts, as described in the paper at
[here](https://eprint.iacr.org/2018/209).
![Workflow Overview](Calypso-ocs-access.png?raw=true "Workflow Overview")

1. Access Control Cothority - implemented using Byzcoin in [Calypso](../calypso/README.md)
2. Secret Management Cothority - implemented in this directory

There is a simple demo the functionality of the system:
[OCS Demo](demo/README.md)

# Protocols

The onet-framework uses protocols at its lowest level to define communication
patterns betwen nodes. We use two protocols in the onchain-secrets service:

- [DKG](../dkg/DKG.md) - Distributed Key Generation, an implementation of
  the following paper: "Secure Distributed Key Generation for Discrete-Log
  Based Cryptosystems" by R. Gennaro, S. Jarecki, H. Krawczyk, and T. Rabin.
- [ocs](Renecrypt.md) - the long-term secrets version of the on-chain secrets
  protocol with server-side secret reconstruction described in
  [CALYPSO](https://eprint.iacr.org/2018/209.pdf).

## Distributed Key Generation

The DKG protocol creates random shares of a secret key that are only
stored at each node. Together these nodes create a public shared key
without creating the secret shared key. As a group they can encrypt
and decrypt data without the need to create the secret shared key,
but with each node participating in part of the encryption or
decryption. For more information, please see [here](../dkg/DKG.md).

## Onchain-Secrets

Based on the DKG, data that is ElGamal encrypted using the public
shared key from the DKG can be re-encrypted under another public key
without the data being in the clear at any given moment. This is used
in the onchain-secrets skipchain when a reader wants to recover the
symmetric key.

# Variables used

When going through the code, the variables follow the CALYPSO paper
in the Appendix B under **Secret reconstruction at the trusted server**
as far as possible.

Here is a short recap of the different variable-names used in the
re-encryption:

- X: the aggregate public key of the OCS (LTS), also used as the
ID of the OCS
- C: the ElGamal part of the data, with maximal key-length of 232 bits for
Ed25519
- U: the encrypted random value for the ElGamal encryption, the commit
- Xc: the public key of the reader under which U will be re-encrypted
- XHatEnc: the re-encrypted random value for the ElGamal encryption

# API

This onet-service offers an API to interact with the OnChain-Secrets service
and allows you to:

- `AddPolicyCreateOCS` - define who is allowed to create new OCS-instances
- `CreateOCS` - start a new OCS instance
- `GetProofs` - returns a list of signatures from nodes on an OCS-instance
- `Reencrypt` - request a reencryption on an encrypted secret
- `Reshare` - not implemented - re-define the set of nodes holding an OCS-instance

For all the policies and authentications, different Access Control Cothorities
can be defined. Currently the two following ACCs are defined:

- `ByzCoin` - using DARCs to define access control
- `X509Cert` - using x509-certs to define who is allowed to access the system

## AddPolicyCreateOCS

This is the entry point to the OCS system. Every node needs to define under
which policy he accepts new OCS instances. For the two ACCs, this is
defined as follows:

- `ByzCoin` - by giving a byzcoin-ID, CreateOCS will accept every proof of 
an LTSInstance that can be verified using a stored byzcoin-ID
- `X509Cert` - by giving a root-CA, CreateOCS will accept every request with
policies signed by this root-CA 

## CreateOCS

Using the CreateOCS endpoint, a client can request the system to set up a
new OCS instance. The request is only accepted if the policy of one of the
ACCs is fulfilled:

- `ByzCoin` - the proof given in the `Reencrypt` policy must be verifiable
with one of the stored byzcoin-IDs
- `X509Cert` - the certificate given in `Reencrypt` and `Reshare` must have
been signed by one of the root-CAs 

The CreateOCS service endpoint returns a `LTSID` in the form of a 32 byte
slice. This ID represents the group that created the distributed key. Any node
can participate in as many DKGs as you want and will get a random `LTSID`
assigned.

## GetProofs

Once an OCS instance has been created, this API endpoint can be called. The
contacted node will send a request to sign the OCS-identity to all other nodes 
and then return a list of all signatures, one per node.

These signatures can be verified to make sure that the OCS-instance has been
correctly set up and that the node contacted in `CreateOCS` didn't change
the roster. 

## Reshare - not yet implemented

It is possible that the roster might change and the LTS shares must be
re-distributed but without changing the LTS itself. We accomplish this in two
steps.

1. The authorised client(s) must update the LTS roster in the blockchain (an
   instance of the LTS smart contract).
2. Then, the client instructs the calypso conodes to run the resharing
   protocol. The nodes in the new roster find and check the proof of
   roster-change in ByzCoin, and then start the protocol to reshare the secret
   between themselves.

For this operation, all nodes must be online. By default, a threshold of 2/3 of
the nodes must be present for the decryption.