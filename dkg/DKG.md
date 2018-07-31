Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../../README.md) ::
[Building Blocks](../../doc/BuildingBlocks.md) ::
Distributed Key Generation

# Distributed Key Generation

Package dkg implements the protocol described in
"Secure Distributed Key Generation for Discrete-Log
Based Cryptosystems" by R. Gennaro, S. Jarecki, H. Krawczyk, and T. Rabin.
DKG enables a group of participants to generate a distributed key
with each participants holding only a share of the key. The key is also
never computed locally but generated distributively whereas the public part
of the key is known by every participants.

Instead of using discrete-log cryptosystem this implementation also works very
well with elliptic curves.

The underlying basis for this protocol is in the kyber-library:
https://github.com/dedis/kyber/tree/master/share/dkg/rabin

## Research Paper

- [Secure Distributed Key Generation for Discrete-Log Based Cryptosystems](http://groups.csail.mit.edu/cis/pubs/stasio/vss.ps.gz)
