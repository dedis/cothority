Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../../README.md) ::
[Applications](../../doc/Applications.md) ::
[Onchain Secrets](../README.md) ::
Protocols

# Protocols

The onet-framework uses protocols at its lowest level to define communication
patterns betwen nodes. We use two protocols in the onchain-secrets service:

- [DKG](../../dkg/DKG.md) - Distributed Key Generation, an implementation of
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
decryption. For more information, please see [here](../../dkg/DKG.md).

## Onchain-Secrets

Based on the DKG, data that is ElGamal encrypted using the public
shared key from the DKG can be re-encrypted under another public key
without the data being in the clear at any given moment. This is used
in the onchain-secrets skipchain when a reader wants to recover the
symmetric key.
