Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Applications

# Applications

An application is a higher level of abstraction and fulfils a purpose that is
bigger than that of a library. It is not used by any other building block or
application, but rather uses building blocks itself.

It is difficult to draw a clear line between libraries and applications in the
cothority repository. This is because even the libraries can have a command
line interface to interact between a client and the library. So the presence
of a CLI is not enough to be an application, while it is a requirement.

The currently elected applications in the cothority are:
- [Status Report](../status/README.md) reports the status of a node
- [CISC Identity SkipChain](../cisc/README.md) stores key/value keypairs on a
skipchain, has special modules for handling ssh-keys, storing webpages and
requesting certificates from letsencrypt
- [Onchain-Secrets](../ocs/README.md) hides data on a blockchain and adds
an access control to it
- [Proof of Personhood](../pop/README.md) create a PoP party to distribute unique
cryptographic tokens to physical people
- [E-voting](../evoting/README.md) following Helios to store votes on a blockchain,
shuffle them and decrypt all votes

Another block that is on the very edge of application and building block is the
[skipchain](../skipchain/README.md). It's more than a building block, because it
already has some functionality. But it's not an application, because you cannot
do anything useful with it as-is.
