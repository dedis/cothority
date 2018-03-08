Navigation: [DEDIS](https://github.com/dedis/doc) ::
[Cothority](https://github.com/dedis/cothority) ::
Applications

# Applications

It is difficult to draw a clear line between libraries and applications in the
cothority repository. This is because even the libraries can have a command
line interface to interact between a client and the library. So the presence
of a CLI is not enough to be an application, while it is a requirement.

An application is a higher level of abstraction and fulfils a purpose that is
bigger than that of a library. It is not used by any other building block or
application, but rather uses building blocks itself.

The currently elected applications in the cothority are:
- [Onchain-Secrets](../ocs/README.md)
- [Proof of Personhood](../pop/README.md)
- [CISC Identity SkipChain](../cisc/README.md)
- [E-voting](../evoting/README.md)
- [Status Report](../status/README.md)

Another block that is on the very edge of application and building block is the
[skipchain](../skipchain/README.md). It's more than a building block, because it
already has some functionality. But it's not an application, because you cannot
do anything useful with it as-is...
