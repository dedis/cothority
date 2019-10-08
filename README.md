[![Build Status](https://travis-ci.org/dedis/cothority.svg?branch=master)](https://travis-ci.org/dedis/cothority) [![Coverage Status](https://coveralls.io/repos/github/dedis/cothority/badge.svg)](https://coveralls.io/github/dedis/cothority)

Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
Cothority

# Cothority

The collective authority (cothority) project provides a framework for
development, analysis, and deployment of decentralized, distributed
(cryptographic) protocols. A given set of servers running these protocols is
referred to as a _collective authority_ or _cothority_. Individual servers are
called _cothority servers_ or _conodes_. The code in this repository allows you
to access the services of a cothority and/or run your own conode. The cothority
project is developed and maintained by the [DEDIS](http://dedis.epfl.ch) lab at
[EPFL](https://epfl.ch).

This is an overview of this README:
- [Documentation](#documentation) with links to different parts of the cothority
  - [Topically ordered](#topically-ordered) explains the different functional
    pieces from the cothority from a research point of view
  - [Network ordered](#network-ordered) gives an overview from a networking point
    of view
  - [Links](doc/README.md) collects
  links to other repositories, papers and webpages that are important to the cothority.
- [1st steps](#first-steps) giving two example apps ready to use
- [Participating](#participating-in-the-cothority) on how to help us getting cothority even better
  - [Setting up your own conode](#setting-up-your-own-conode) describes why you
    might want to set up your own conode

Don't forget that the cothority is part of a [bigger
environment](https://github.com/dedis/doc/tree/master/README.md).

## Versioning and Roadmap

We use a yearly release cycle, with new releases arriving in February, in time
for students to use a new stable release during the spring semester.

We use semantic versioning, and Go modules to make it possible to develop
from a specific version and know the exact dependencies, and know when
you are about to opt-in to an API-breaking change (because the major version
of one or more of your dependencies changes).

We maintain a major version for 18 months.

The current major version is v3. It was released in Feb 2019. It will receive
security updates, and possibly backports of simple and essential features
from the master branch until June 2020.

The last major version was v2, which is end of life as of June 2019.

As of Oct 2019, work is starting on v4. It will be released in Feb 2020, with
an end of life in June 2021.

As a general rule, the current and last versions of Go are tested and expected
to work to compile Cothority. If you encounter problems with older versions of
the Go toolchain, please report them via a Github issue and we will try to solve
them on a best-effort basis.

### Release v3.1.0

The release introduces the notion of signature scheme for a given skipchain so that
one can define which co-signing algorithm will be used to sign the forward links. This
was necessary in the context of weaknesses in the BLS signature algorithm (see the
[paper](https://crypto.stanford.edu/~dabo/pubs/papers/BLSmultisig.html)). New
skipchains will be created with [BDN](https://github.com/dedis/kyber/tree/master/sign/bdn)
set as the signature scheme.

Because of a new scheme is default, that means that skipchains created after v3.1.0
won't work with older versions as they are not aware of the new scheme. However,
existing skipchains will continue to operate normally. In summary, if you need to
create skipchains after updating to v3.1.0, make sure every conode is at least using
v3.0.1 aswell.

### Release v3.2.0

A new field has been added to the *DataHeader*, *Version*, so that new features or
upgrades can be coordinated between the conodes to only start using it when enough
of them are up to date. The leader will propose a change of version when it detects
that enough of the participants can reach a consensus. A successful increase of
version is announced by an empty block that will act as a barrier between the
previous and the new version. Its *DataHeader* data will contain the new version.

When creating a ledger, the default version is the most recent one and blocks are
continously created with the previous block version until the leader proposes an
upgrade. Note that the initial version is zero for backwards compatibility.

Another important change for this version is about how transactions are created as
they need to include the ByzCoin version to use the correct hash function. The
initial version of the hash was not taking the invoke command into account and it
has been fixed for version one and higher. See below examples:

Go
```go
client := byzcoin.NewClient(id, roster)
tx := client.CreateTransaction(instr1, instr2)
```

Java
```java
ClientTransaction tx = new ClientTransaction(instrs, rpc.getProtocolVersion());
```

Javascript/Typescript
```javascript
const tx = ClientTransaction.make(rpc.getProtocolVersion(), instr1, instr2);
```

### Release v3.3.0

An *experimental* contract has been added to ByzCoin making it possible to use
Ethereum contracts. See directory `bevm`.

The ByzCoin client-side API version number has changed from 1 to 2. Callers
should use the new version in their requests, but the change is backwards
compatible and old clients will still work.

# Documentation

The goal of the cothority is to collect projects that
handle aspects of collective authorities to support scalable, self-organizing
communities. In this document we present the apps that are directly runnable
from the cothority, as well as links to the services and protocols used in
the cothority.

A cothority is a set of conodes that work together to offer collective
authority services. This is used to create distributed and decentralized
services where no single point of failure can put the whole system in jeopardy.
Conodes communicate with each other using protocols that are short-lived, specific
tasks with an outcome that can be read by services. Each conode has a set of
services that can be contacted through a well-defined API from the outside. The
communication through the API is done with a homebrewn protobuf over websockets
implementation.

## Topically ordered

When looking at the cothority modules from a topical point of view, we can break
it up as follows:

```ascii art
+--------------------------+------------+--------------------------+
|                          |APPLICATIONS|                          |
|     Onchain-Secrets      +------------+     Proof of Personhood  |
|                                                                  |
|       ByzCoin                 Status            E-voting         |
+------------------------+---------------+-------------------------+
|                        |BUILDING BLOCKS|                         |
| Consensus              +---------------+       Key Sharding      |
|  - Skipchain                                    - Re-encryption  |
|  - BFT         General Crypto    Messaging      - Decryption     |
|  - Collective   - Neff Shuffle    - Broadcast   - Distributed    |
|    Signing      - RandHerd        - Propagate     Key Generation |
|                 - RandHound                                      |
|                                                                  |
+------------------------------------------------------------------+
```

### Applications

Applications in cothority use different libraries and are the highest level
of abstraction in our framework.

Here you get [a list of all applications in the cothority](doc/Applications.md).

There is one very special application that is considered apart - it's the conode
itself, which holds all protocols and services and can be run as a service on
a server.

[What a Conode can do for you](conode/README.md)

### Building Blocks

Building blocks are grouped in terms of research units and don't have a special
representation in code. Also, the _General Crypto_ library is more of a
collection of different protocols.

Here you get [a list of all building blocks in the cothority](doc/BuildingBlocks.md).

## Network Ordered

If we look at the cothority from a purely networking point of view, we can
describe it as follows:

```ascii art
              +-----------------+                 
              |CLI, JavaScript, | Frontend        
              |Java             |                 
+-------------+-----------------+                 
| Conode,     | Services        | Client to Conode
| Simulations |-----------------+                 
|             | Protocols       | Conode to Conode
+-------------+-----------------+                 
```

### Command Line Interfaces

Command line interfaces (CLI) are used to communicate with one or more conodes.
All CLIs need to have one or more conodes installed.
For the two CLIs in [first steps](#first-steps), you can use the running conodes
at EPFL/DEDIS. If you want to test the other CLIs, you might need to set up
a small network (often 3 nodes is enough) of conodes on your local computer
to test it.

Here you get [a list of all available CLIs in the cothority](doc/CLIs.md).

### Services

Every app communicates with one or more services to interact with one or more
conodes. This is a list of available services that can be contacted on a running
conode. Some of the services are open to all, while others might require authentication
through a PIN or a public key. Most of the apps and services have the same name,
but some are not available as an app or have more than one app using it.

Here you get [a list of all available services in the cothority](doc/Services.md).

### Protocols

Protocols are used to communicate between conodes and execute cryptographic
exchanges. This can be to form a collective signature, create a consensus on a
new state, or simply to propagate a new block from a skipchain. Some protocols
are useful for different services, while others are very service-specific.
Most of the protocols have a paper that is describing how the protocol should
perform and that compares it to other protocols.

Here you get [a list of all available protocols in the cothority](doc/Protocols.md).

### Simulations

Cothority grew up as a research instrument, so one of its advantages is to have
a framework to create simulations and running them locally or on remote servers.
Some of the protocols presented here do have the simulation code. Check it out
here: [Cothority Simulations](doc/Simulation.md).

# First steps

If you're just curious how things work, you can check the status of our test
network or create a collective signature using our running nodes:

## Status

To get the status of the conodes in the cothority, first install the `status` binary:

```
go install ./status
```

Now you can run it by giving the definition of the dedis-cothority on the command line:

```
status -g dedis-cothority.toml
```

# Participating in the cothority

There are different ways to participate in the cothority project. A first step
is to simply test the different CLI applications in this repository and tell us
what were your difficulties or what you would like to use them for.

A next step is to set up your own conode and participate in consensus
operations on skipchains or ledgers.

## Setting up your own conode

A conode is a server program that includes all protocols and services. It can
be run on a public facing server, but for testing purposes it's also possible
to set up a network on a machine and only let it be accessed locally.

- [conode](conode/README.md) is the cothority
server, a special app that includes all services and protocols.
- [How to run a conode](conode/Operating.md)
gives an overview of the environment where a conode can be run
- [DEDIS-cothority](doc/Join.md)
explains how to join the DEDIS-cothority

## Contributing

If you want to contribute to Cothority-ONet, please have a look at
[CONTRIBUTION](CONTRIBUTION.md) for
licensing details. Once you are OK with those, you can have a look at our
coding-guidelines in
[Coding](https://github.com/dedis/Coding). In short, we use the github-issues
to communicate and pull-requests to do code-review. Travis makes sure that
everything goes smoothly. And we'd like to have good code-coverage.

## License

The software in this repository is put under a dual-licensing scheme: In general
all of the provided code is open source via [GNU/AGPL
3.0](https://www.gnu.org/licenses/agpl-3.0.en.html), please see the
[LICENSE](LICENSE.AGPL) file for more details. If you would like to
use Cothority in a way not allowed by the applicable license, please [contact
us](mailto:dedis@epfl.ch) to inquire about conditions to get a commercial license.

## Contact

We are always happy to hear about your experiences with the cothority project.
Feel free to contact us on our
[mailing list](https://groups.google.com/forum/#!forum/cothority) or by
[email](mailto:dedis@epfl.ch).

## Reporting security problems

This library is offered as-is, and without a guarantee. It will need an
independent security review before it should be considered ready for use in
security-critical applications. If you integrate Cothority into your application it
is YOUR RESPONSIBILITY to arrange for that audit.

If you notice a possible security problem, please report it
to dedis-security@epfl.ch.

# Who is using our code?

This is a list of people outside of DEDIS who is using our codebase for research
or applied projects. If you have an interesting project that you would like to
have listed here, please contact us at [dedis@epfl.ch](mailto:dedis@epfl.ch).

- [Unlynx](https://github.com/lca1/unlynx) - A decentralized privacy-preserving data sharing tool
- [Medco](https://github.com/lca1/medco) - Privacy preserving medical data sharing
- [ByzGen](http://byzgen.com/) - Tracking and secure storage of digital and hard assets
- [PDCi2b2](https://github.com/JLRgithub/PDCi2b2) - Private Data Characterization for [i2b2](https://www.i2b2.org/)
