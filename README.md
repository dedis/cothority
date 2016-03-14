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

In order to build (and run) the simulations you need to install a recent 
[Golang](https://golang.org/dl/) version (1.5.2+).
See Golang's documentation on how-to 
[install and configure](https://golang.org/doc/install) Go (including 
setting up a GOPATH environment variable). 
    
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
more information on how to use [DeterLab](Deterlab.md)

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

## RandHound - Verifiable Randomness Scavenging Protocol 

RandHound is a novel protocol for generating strong, bias-resistant, 
public random numbers in a distributed way and produces in parallel a 
proof to convince third parties that the randomness is correct and 
unbiased, provided a threshold of servers are non-malicious.

## JVSS - Joint Verifiable Secret Sharing

The JVSS protocol implements Schnorr signing using joint 
[verifiable](http://ieeexplore.ieee.org/xpls/abs_all.jsp?arnumber=4568297&tag=1) 
[secret sharing](http://link.springer.com/chapter/10.1007%2F3-540-68339-9_17).


## Naive and NTree

Similar to JVSS these two protocols are included to compare their 
scalability with CoSi's. 
In the naive approach a leader simply collects standard individual 
signatures of all participants. 
NTree is the same protocol but using a tree (n-ary) topology for 
aggregating the individual signatures.

# SDA framework

Core of this repository is a framework for implementing secure, 
distributed systems. 
It does so by offering an API for implementing and running different 
kind of protocols which may rely on other, pre-defined protocols.
 
Using the SDA-cothority framework, you can:

* simulate up to 8192 nodes using Deterlab (which is based on 
[PlanetLab](https://www.planet-lab.org/))
* run local simulations for up to as many nodes as your local machines
allows

The framework is round-based using message-passing between different 
hosts which form a tree. Every protocol defines the steps needed to 
accomplish the calculations, and the framework makes sure that all 
messages are passed between the hosts.
  
The directory-structure is as follows:

* [`lib/`](lib/): contains all internally used libraries
* [`lib/sda/`](lib/sda/): basic definition of our framework
* [`protocols/`](protocols/): one directory per protocol, holds both the 
definition and eventual initialisation necessary for a simulation
* [`simul/`](simul/): used for running simulations on localhost and DeterLab
