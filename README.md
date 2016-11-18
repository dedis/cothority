[![Build Status](https://travis-ci.org/dedis/cothority.svg?branch=master)](https://travis-ci.org/dedis/cothority)
[![Coverage Status](https://coveralls.io/repos/github/dedis/cothority/badge.svg)](https://coveralls.io/github/dedis/cothority)

# Cothority

This repository implements the collective authority (cothority) framework.
It offers a framework for simulating and deploying decentralized and 
distributed cryptographic protocols.

It works closely together with the cryptographic-library found in `dedis/crypto`
and allows for setting up of *protocols*, *services*, and "apps". A protocol will
send back and forth messages, mostly in a tree-based structure of
nodes, but it can also broadcast or bypass the tree.
A service interacts with clients and will spawn and wait for the result
of different protocols.
An app is an example of a user-space program that can communicate to one or more
services of a cothority.
You can find a list of protocols and services supported later.

## Warning
**The software provided in this repository is highly experimental and under
heavy development. Do not use it yet for anything security-critical.  or if you
use it, do so in a way that supplements (rather than replacing) existing, stable
signing mechanisms.

All usage is at your own risk!**

## Requirements

In order to build (and run) the simulations you need to install a recent 
[Golang](https://golang.org/dl/) version (1.5.2+).
See Golang's documentation on how-to 
[install and configure](https://golang.org/doc/install) Go,
including setting the GOPATH environment variable. 
You can run CoSi either as a standalone application or in testbed simulations,
as described below. 

## Versions

For the moment we have two version: _v0_ and _master_.

### V0

This is a stable version that depends on the v0-versions of the other dedis-packages. It will only receive bugfixes, but no changes that will make the code incompatible. You can find this version at:

https://github.com/dedis/cothority/tree/v0

If you write code that uses our library in the v0-version, be sure to reference it as

```
import "gopkg.in/dedis/cothority.v0"
```

### Master

The master-branch is used for day-to-day development and will break your code about once a week. If you are using this branch, be sure to do

```
go get -u -t ./...
```

from time to time, as all dedis-dependencies change quite often.

# Installation

There are three apps available:

* [cothorityd](https://github.com/dedis/cothority/app/cothorityd) - which is the server-part that you can run to add a node
* [CoSi](https://github.com/dedis/cothority/app/cosi) - the CoSi-app
* [status](https://github.com/dedis/cothority/app/status) - reads out the status of a cothority

You will find a README.md in each of its directory. To build the apps, you can
run the following commands:

```
go get github.com/dedis/cothority/app/cothorityd
go get github.com/dedis/cothority/app/status
```

# Apps

* [cothorityd](app/cothorityd) - the basic 
* [cosi](app/cosi) - collective signatures
* [status](app/status) - returns the status of the given group
* [cisc](app/cisc) - handle your ssh-keys on a blockchain
* [hotpets](https://github.com/dedis/cothority/tree/hpets16/app/cisc) - hotpets16-branch

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

# Simulation
Starting a simulation of one the provided protocols (or your own) either 
on localhost or, if you have access, on [DeterLab](https://www.isi.deterlab.net) 
is straight forward and described in the following sub-sections.

## Localhost
To run a simple signing check on localhost, execute the following 
commands:

```bash
# download project and its dependencies
go get -d github.com/dedis/cothority 
# build the simulation binary
cd $GOPATH/src/github.com/dedis/cothority/simul
go build
# run the simulation
./simul runfiles/test_cosi.toml
```

## DeterLab

For more realistic, large scale simulations you can use DeterLab. 
Find more information on how to use [DeterLab here](Deterlab.md).

# SDA framework

Core of this repository is a framework for implementing secure, 
distributed systems. 
It does so by offering an API for implementing and running different 
kind of protocols which may rely on other, pre-defined protocols.
 
Using the SDA-cothority framework, you can:

* simulate up to 32000 nodes using Deterlab (which is based on 
[PlanetLab](https://www.planet-lab.org/))
* run local simulations for up to as many nodes as your local machines
allows

The framework is round-based using message-passing between different 
hosts which form a tree. Every protocol defines the steps needed to 
accomplish the calculations, and the framework makes sure that all 
messages are passed between the hosts.
  
## Directory-structure

* [`sda/`](sda/): basic definition of our framework
* `crypto/`, `log/`, `monitor/`, `network/`: additional libraries for the framework
* [`simul/`](simul/): simulation-related code
* [`app/`](app/): all apps in user-space
* [`protocols/`](protocols/): the protocol-definitions for cothority
* [`services/`](services/): services using the protocols
