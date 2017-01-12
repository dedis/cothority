[![Build Status](https://travis-ci.org/dedis/cothority.svg?branch=master)](https://travis-ci.org/dedis/cothority)
[![Coverage Status](https://coveralls.io/repos/github/dedis/cothority/badge.svg)](https://coveralls.io/github/dedis/cothority)


# Cothority

The collective authority (cothority) project provides a framework for development, analysis, and deployment of decentralized, distributed (cryptographic) protocols. A given set of servers that runs these protocols is referred to as a *collective authority* or *cothority*. Individual servers are called *cothority servers* or *conodes*. The code in this repository allows you to access the services of a cothority as well as to run your own conode.

The cothority project is developed and maintained by the [DEDIS](http://dedis.epfl.ch) lab at [EPFL](https://epfl.ch). The research behind the project has been published in several academic papers:

- **Keeping Authorities “Honest or Bust” with Decentralized Witness Cosigning** ([pdf](http://arxiv.org/pdf/1503.08768.pdf)); *Ewa Syta, Iulia Tamas, Dylan Visher, David Isaac Wolinsky, Philipp Jovanovic, Linus Gasser, Nicolas Gailly, Ismail Khoffi, Bryan Ford*; IEEE Symposium on Security and Privacy, 2016. 
- **Enhancing Bitcoin Security and Performance with Strong Consistency via Collective Signing** ([pdf](https://www.usenix.org/system/files/conference/usenixsecurity16/sec16_paper_kokoris-kogias.pdf)); *Eleftherios Kokoris-Kogias, Philipp Jovanovic, Nicolas Gailly, Ismail Khoffi, Linus Gasser, Bryan Ford*; USENIX Security, 2016.
- **Scalable Bias-Resistant Distributed Randomness** ([pdf](https://eprint.iacr.org/2016/1067.pdf)); *Ewa Syta, Philipp Jovanovic, Eleftherios Kokoris Kogias, Nicolas Gailly, Linus Gasser, Ismail Khoffi, Michael J. Fischer, Bryan Ford*; IACR Cryptology ePrint Archive, Report 2016/1067.


## Disclaimer 

The software in this repository is highly experimental and under heavy development. Do not use it for anything security-critical yet.

**All usage is at your own risk**!

## Usage

To use the code of this repository make sure that you have a working [Golang installation](https://golang.org/doc/install) and that the
[`$GOPATH`](https://golang.org/doc/code.html#GOPATH) variable is set on your system. You have two options to use the cothority code:

1. [Accessing cothority services through a client](https://github.com/dedis/cothority#accessing-cothority-services-through-a-client)
2. [Setting up and running a cothority server](https://github.com/dedis/cothority#setting-up-and-running-a-cothority-server)

**Note:** The main dependencies include the following libraries, all of which are developed and maintained by DEDIS:

- Network: [dedis/onet](https://github.com/dedis/onet)
- Cryptography: [dedis/crypto](https://github.com/dedis/crypto)
- Protobuf: [dedis/protobuf](https://github.com/dedis/protobuf)


### Accessing Cothority Services Through a Client

A cothority provides several [services](https://github.com/dedis/cothority/wiki/Apps) to its clients. As an example, we illustrate how a client can use a cothority to generate a collective (Schnorr) signature on a file using the CoSi protocol. For more details on CoSi, refer to the [research paper](https://arxiv.org/pdf/1503.08768.pdf).

#### Compiling the CoSi Client

To build and install the CoSi client, execute:

```
go get -u github.com/dedis/cothority/cosi
```

#### Using the CoSi Client

To tell the CoSi client which cothority it should use for signing requests, you need to specify a configuration file. One option is to use the [DEDIS cothority configuration file](https://github.com/dedis/cothority/blob/master/dedis-servers.toml) which is included in this repository. To have a shortcut for later on, set:

```
export COTHORITY=$GOPATH/src/github.com/dedis/cothority/dedis-cothority.toml 
```

To request a collective (Schnorr) signature `file.sig` on a `file` from the DEDIS cothority, use:

```
cosi sign -g $COTHORITY -o file.sig file
```

To verify a collective (Schnorr) signature `file.sig` of the `file`, use:

```
cosi verify -g $COTHORITY -s file.sig file
```

**Note:** Although signature verification is done locally, you need to provide a cothority configuration file to tell the CoSi client which public key to use.

### Setting up and Running a Cothority Server

#### Compiling the Cothority Server

To build and install the conode binary execute the following sequence of commands:

```
go get -u github.com/dedis/cothority/conode
```

To get an overview on the functionality of a conode, type:

```
conode help
```

#### Configuring the Cothority Server

To configure your conode you need to *open two consecutive ports* (e.g., 6879 and 6880) on your machine, then execute

```
conode setup
```

and follow the instructions of the dialog. After a successful setup there should be two configuration files:

- The *public configuration file* of your cothority server is located at `~/.config/conode/public.toml`. Adapt the `description` variable to your liking and send the file to other cothority operators to request access to the cothority. 
- The *private configuration file* of your cothoriy server is located at `~/.config/conode/private.toml`.

**Warning:** Never (!!!) share the file `private.toml` with anybody, as it contains the private key of your conode.

**Note:** The [public configuration file](https://github.com/dedis/cothority/blob/master/dedis-servers.toml) of the DEDIS cothority provides an example of how such a file with multiple conodes usually looks like.

#### Running the Cothority Server

To start your conode with the default (private) configuration file, located at `~/.config/conode/private.toml`, execute:

```
conode
```

To increase the verbosity of your conode, start it with:

```
cothority -d 3
```

To use a (private) configuration file at a custom location, start your conode via:

```
cothority -config path/to/private.toml
```

## Documentation

Each of the parts of the cothority project has a corresponding wiki which are worth checking out if you are interested in more details:

- The [cothority wiki](https://github.com/dedis/cothority/wiki) provides an overview on supported protocols, services, and applications.
- The [cothority template wiki](https://github.com/dedis/cothority_template/wiki) shows how you can develop your own protocols, services, and applications such that they can be integrated into the cothority project.
- The [cothority network library wiki](https://github.com/dedis/onet/wiki) presents details on the inner workings of the cothority framework.

If you are writing code for the project, make sure to have a look at our [coding guidelines](https://github.com/dedis/Coding).

## Contributing

If you are interested in contributing to the cothority project, please check our guidlines found in [CONTRIBUTION](https://github.com/dedis/cothority/blob/master/CONTRIBUTION), [CLAC](https://github.com/dedis/cothority/blob/master/CLAC), and [CLAI](https://github.com/dedis/cothority/blob/master/CLAI).

## License

The software in this repository is put under a dual-licensing scheme: In general all of the provided code is open source via [GNU/AGPL 3.0](https://www.gnu.org/licenses/agpl-3.0.en.html), please see the [LICENSE](https://github.com/dedis/cothority/blob/master/LICENSE.AGPL) file for more details. If you intend to use the cothority code for commercial purposes, please [contact us](mailto:contact@dedis.epfl.ch) to get a commercial license.

## Contact

We are always happy to hear about your experiences with the cothority project. Feel free to contact us on our [mailing list](https://groups.google.com/forum/#!forum/cothority) or by [email](mailto:contact@dedis.epfl.ch).

