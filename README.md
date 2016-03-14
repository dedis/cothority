# Cothority

This repository provides an implementation for the prototype of the 
collective authority (cothority) framework. 
The system is based on CoSi, a novel protocol for collective signing 
which itself builds upon Merkle trees and Schnorr multi-signatures over 
elliptic curves. 
CoSi enables authorities to have their statements collectively signed 
(co-signed) by a diverse, decentralized, and scalable group of 
(potentially thousands of) witnesses and, for example, could be employed 
to proactively harden critical Internet authorities. 
Among other things, one could imagine applications to the Certificate 
Transparency project, DNSSEC, software distribution, the Tor anonymity 
network or cryptocurrencies.

## Further Information

* Keeping Authorities "Honest or Bust" with Decentralized Witness 
Cosigning: [paper](http://arxiv.org/abs/1503.08768), 
[slides](http://dedis.cs.yale.edu/dissent/pres/151009-stanford-cothorities.pdf)
* Certificate Cothority - Towards Trustworthy Collective CAs: 
[paper](https://petsymposium.org/2015/papers/syta-cc-hotpets2015.pdf)
* For questions and discussions please refer to our 
[mailing list](https://groups.google.com/forum/#!forum/cothority).

## Warning
**The software provided in this repository is highly experimental and 
under heavy development. Do not use it for anything security-critical. 
All usage is at your own risk!**

## Requirements

In order to build (and run) the simulations you need to install 
[Golang](https://golang.org/dl/).
See Golang's documentation on how-to 
[install and configure](https://golang.org/doc/install) Go (including 
setting up a GOPATH environment variable).

### Main Dependencies 

* [dedis/crypto](https://github.com/dedis/crypto)
* [dedis/protobuf](https://github.com/dedis/protobuf)

* If you are interested in a full for a full list of dependencies
`go list -f '{{ join .Deps  "\n"}}' .` in the top-level project directory
    
# Simulation
It is very easy to start a simulation of the provided (or your own) 
protocols either on localhost or, if you have access, on 
[DeterLab](https://www.isi.deterlab.net).

## Localhost
To run a simple signing check on localhost, execute the following 
commands:

```
$ go get -d github.com/dedis/cothority # download project and its dependencies
$ cd $GOPATH/src/github.com/dedis/cothority/simul
$ go build
$ ./simul runfiles/test_cosi.toml
```

## DeterLab

For large scale simulations you can run simulations on DeterLab. Find 
more information [here](Deterlab.md)

# Protocols

## CoSi - Collective Signing

[CoSi](http://dedis.cs.yale.edu/dissent/papers/witness-abs) is a 
protocol for scalable collective signing, which enables an authority or 
leader to request that statements be publicly validated and (co-signed) 
by a decentralized group of witnesses. 
Each run of the protocol yields a single digital signature with size and 
verification cost comparable to an individual signature, but compactly
attests that both the leader and perhaps many witnesses observed and 
agreed to sign the statement.

## JVSS - Joint Verifiable Secret Sharing

A textbook Shamir signing for baseline-comparison against the collective 
signing protocol.

## RandHound - Verifiable Randomness Scavenging Protocol 

RandHound is a novel protocol for generating strong, bias-resistant, 
public random numbers in a distributed way and produces in parallel a 
proof to convince third parties that the randomness is correct and 
unbiased provided a threshold of servers are non-malicious.


## Distribution
* Planned:
    * Binary standalone application
    * Docker image

## SDA framework

Core of this repository is a framework for implementing secure, 
distributed systems. 
It does so by offering an API for implementing and running different 
kind of protocols which may rely on other, pre-defined protocols.
 
Using the SDA-cothority framework, you can:

* Simulate up to 8192 nodes using Deterlab (which is based on Planetlab)
* Run local simulations for up to 128 nodes (restricted by your computer)
* Distribute binaries for real-world deployment (work in progress).

The framework is round-based using message-passing between different 
hosts which form a tree. Every protocol defines the steps needed to 
accomplish the calculations, and the framework makes sure that all 
messages are passed between the hosts.
  
The directory-structure is as follows:

* `lib/` - holding all internally used libraries
* `lib/sda/` - the basic definition of our framework
* `protocols/` - one directory per protocol, holds both the definition 
and eventual initialisation needed for simulation
* `simul/` - used for running simulations on localhost and DeterLab
* `dist/` - creates distributable binaries, in .tgz or Docker-format
