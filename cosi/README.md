Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
Collective Signing

# Collective Signing (CoSi)

*WARNING*: this package is kept here for historical and research purposes. It
should not be used in other services as it has been deprecated by the
[blscosi](../blscosi)
package.

CoSi is a protocol that enables a decentralized (potentially large) group of
independent servers to efficiently issue aggregate Schnorr signatures. These
collective signatures (*co-signatures*) convey the same information as a list of
conventional signatures but are much more compact and efficient to verify against
the aggregate public key of the server group. In practice, a co-signature is not
much bigger than an individual Schnorr signature.

CoSi is intended to facilitate increased transparency and security-hardening
for critical Internet authorities such as certificate authorities,
[time services](http://www.nist.gov/pml/div688/grp40/its.cfm),
naming authorities such as [DNSSEC](http://www.dnssec.net),
software distribution and update services,
directory services used by tools such as [Tor](https://www.torproject.org),
and next-generation cryptocurrencies.

# Implementation

CoSi is implemented as a single protocol. It uses four types of messages:
announcement, commitment, challenge and response. The root initializes the
protocol with the announcement message. It is then propagated down the tree,
where the tree is specified by the service or the program that initializes the
protocol. Upon receiving the announcement, the leafs begin propagating
commitment messages back up, which are aggregated at the intermediate nodes.
As a result, the root should receive an aggregate commitment from all the
nodes. Then, the root starts a second round-trip by propagating the challenge
message down the tree. Like before, upon receiving the challenge, the leafs
begin to propagate the response messages back up and are aggregated by the
intermediate nodes. Finally, using the aggregate response, the root generates
a final signature. An illustration is shown below.

```
       root     announcement  ▲    challenge    ▲
       /   \         |        |        |        |
     node  node      |        |        |        |
     /  \    \       |        |        |        |
    /    \    \      |        |        |        |
  leaf  leaf  leaf   ▼    commitment   ▼     response
```

We provide hooks functionality where the initiator of the protocol is able to
add custom behaviour at every stage of the protocol. For instance, the
initiator can create a hook and register it with the final signature such that
the signature is sent back to one of the receiver's channels.

## Research Paper

For further background and technical details, please refer to the
[research paper](https://dedis.cs.yale.edu/dissent/papers/witness-abs/) or one of the following
links:

- [Certificate Cothority - Towards Trustworthy Collective CAs](https://petsymposium.org/2015/papers/syta-cc-hotpets2015.pdf),
[HotPETS 2015](https://petsymposium.org/2015/hotpets.php)
- [Enhancing Bitcoin Security and Performance with Strong Consistency via Collective Signing](https://www.usenix.org/system/files/conference/usenixsecurity16/sec16_paper_kokoris-kogias.pdf)

## Links

- [ftcosi](../ftcosi) is a fault tolerant version of CoSi *Please use ftCoSi*
- [CoSi CLI](CLI.md) is a command line interface for interacting with CoSi
- [CoSi protocol](protocol) the protocol used for collective signing
- [CoSi service](service) the service with the outward looking API
- [CoSi RFC]((https://github.com/dedis/doc/tree/master/README.md)/cosi) a draft for a CoSi RFC

## Other Standalone CoSi Clients

- C language, signature verification only: in [temporary fork of libsodium](https://github.com/bford/libsodium).
See the new `crypto_sign_ed25519_verify_cosi` function in the
[crypto_sign/ed25519/ref10](https://github.com/bford/libsodium/blob/master/src/libsodium/crypto_sign/ed25519/ref10/open.c)
module, and the test suites for CoSi signature verification in
[libsodium/test/default/sign.c](https://github.com/bford/libsodium/blob/master/test/default/sign.c).
Run `make check` as usual for libsodium to run all tests including these.
- Go language, verification and signing code: in
[temporary fork of golang.org/x/crypto](https://github.com/bford/golang-x-crypto).
See the new [ed25519/cosi] package, with
[extensive godoc API documentation here](https://godoc.org/github.com/bford/golang-x-crypto/ed25519/cosi).
Run `go test` to run the standard test suite, and `go test -bench=.` to run a
suite of performance benchmarks.
