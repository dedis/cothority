[![Build Status](https://travis-ci.org/dedis/cothority.svg?branch=master)](https://travis-ci.org/dedis/cothority) [![Coverage Status](https://coveralls.io/repos/github/dedis/cothority/badge.svg)](https://coveralls.io/github/dedis/cothority)

# Cothority

WARNING: The master branch is currently unstable, as it is in development phase. If you are starting new work with this repository, use gopkg.in/dedis/cothority.v1 instead. The source code for this stable branch is [here](https://github.com/dedis/cothority/tree/v1.2).

The collective authority (cothority) project provides a framework for development, analysis, and deployment of decentralized, distributed (cryptographic) protocols. A given set of servers running these protocols is referred to as a _collective authority_ or _cothority_. Individual servers are called _cothority servers_ or _conodes_. The code in this repository allows you to access the services of a cothority and/or run your own conode. The cothority project is developed and maintained by the [DEDIS](http://dedis.epfl.ch) lab at [EPFL](https://epfl.ch).

## Disclaimer

The software in this repository is highly experimental and under heavy development. Do not use it for anything security-critical yet.

**All usage is at your own risk**!

## Overview

In addition to our [cothority](https://github.com/dedis/cothority/tree/master/doc/README.md) main page that holds links to all our documentation, you can directly jump in to one of our three wikis to get the information about using the cothority, extending it, or helping in development:

- The [cothority wiki](https://github.com/dedis/cothority/wiki) provides an overview on supported protocols, services, and applications.
- The [cothority template wiki](https://github.com/dedis/cothority_template/wiki) shows how you can develop your own protocols, services, and applications such that they can be integrated into the cothority project.
- The [cothority network library wiki](https://github.com/dedis/onet/wiki) presents details on the inner workings of the cothority framework.

## Getting Started

Very short overview for two steps: one to show the status of the nodes in the DEDIS-cothority, one for signing and verifying a file using all of these nodes.

### Status

To get the status of the conodes in the cothority, first install the status binary:

```go
go get github.com/dedis/cothority/status
export DEDIS_GROUP=$(go env GOPATH)/src/github.com/dedis/cothority/dedis-cothority.toml
```

Now you can run it by giving the definition of the dedis-cothority on the command line:

```go
status $DEDIS_GROUP
```

### Collective Signing

Another service available is collective signing, or CoSi, that asks a set of conodes to create a collective signature on an input data. For installation, type:

```go
go get github.com/dedis/cothority/cosi
export DEDIS_GROUP=$(go env GOPATH)/src/github.com/dedis/cothority/dedis-cothority.toml
```

Now you can create a file and have it signed by the cothority:

```go
date > /tmp/my_file
cosi sign --group $DEDIS_GROUP /tmp/my_file | tee sig.json
```

And later somebody can verify the signature is correct by running the following command:

```go
cosi verify --group dedis-cothority.toml --signature sig.json dedis-cothority.toml
```

If everything is correct, it should print

```
[+] OK: Signature is valid
```

## All apps in the cothority

Name                                                              | Description
----------------------------------------------------------------- | ------------------------------------------------
[`cisc`](https://github.com/dedis/cothority/tree/master/cisc)     | Manage identity skipchains
[`conode`](https://github.com/dedis/cothority/tree/master/conode) | The cothority server
[`cosi`](https://github.com/dedis/cothority/tree/master/cosi)     | Request and verify collective signatures
[`pop`](https://github.com/dedis/cothority/tree/master/)          | Proof of Personhood parties
[`scmgr`](https://github.com/dedis/cothority/tree/master/)        | Skipchain Manager to inspect a running skipchain
[`status`](https://github.com/dedis/cothority/tree/master/status) | Query status of a cothority server

# Contributing

If you are interested in contributing to the cothority project, please check our guidlines found at <CONTRIBUTION>, <CLAC>, and <CLAI>. Make also sure to have a look at our [coding guidelines](https://github.com/dedis/Coding).

# License

The software in this repository is put under a dual-licensing scheme: In general all of the provided code is open source via [GNU/AGPL 3.0](https://www.gnu.org/licenses/agpl-3.0.en.html), please see the [LICENSE](LICENSE.AGPL) file for more details. If you intend to use the cothority code for commercial purposes, please [contact us](mailto:dedis@epfl.ch) to get a commercial license.

# Contact

We are always happy to hear about your experiences with the cothority project. Feel free to contact us on our [mailing list](https://groups.google.com/forum/#!forum/cothority) or by [email](mailto:dedis@epfl.ch).
