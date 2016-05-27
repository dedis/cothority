[![Build Status](https://travis-ci.org/dedis/cothority.svg?branch=master)](https://travis-ci.org/dedis/cothority)

# Cothority

This repository implements a of the collective authority (cothority) framework. 
The system is based on CoSi, a novel protocol for collective signing 
which itself builds on Schnorr multi-signatures over elliptic curves. 
CoSi enables authorities to have their statements collectively signed 
(co-signed) by a diverse, decentralized, and scalable group of 
(potentially thousands of) witnesses and, for example, could be employed 
to proactively harden critical Internet authorities. 
Among other things, one could imagine applications to Certificate 
Transparency, DNSSEC, software distribution, the Tor anonymity 
network, and cryptocurrencies.

## Further Information

Primary information sources:
* Keeping Authorities "Honest or Bust" with Decentralized Witness 
Cosigning: [paper](http://dedis.cs.yale.edu/dissent/papers/witness-abs), 
[slides](http://dedis.cs.yale.edu/dissent/pres/151009-stanford-cothorities.pdf)
* For questions and discussions please join the
[mailing list](https://groups.google.com/forum/#!forum/cothority).

Other cothority-related research papers:
* Certificate Cothority - Towards Trustworthy Collective CAs: 
[paper](https://petsymposium.org/2015/papers/syta-cc-hotpets2015.pdf)
* Enhancing Bitcoin Security and Performance with Strong Consistency via Collective Signing: [paper](http://arxiv.org/abs/1602.06997)
 

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


# Command-line Interface

You can run `cosi`, a simple standalone collective signing application, 
to request a collective signing group you define
to witness and cosign any message you propose.
In the current implementation,
these witnesses do not validate or check the messages you propose
in any way; they merely serve to provide transparency
by publicly attesting the fact that
they have observed and cosigned the message.

## Installation

For convenience we provide x86-64 binaries for Linux and Mac OS X,
which are self-contained and don't require Go to be installed.
But of course you can also compile the tools from source.
 
### Installing binaries from .tar.gz

Download the latest package from 

	https://github.com/dedis/cothority/releases/latest

and untar into a directory that is in your `$PATH`:

```bash
tar xf conode-*tar.gz -C ~/bin
```

### Installing from source

To install the command-line tools from source, make sure that
[Go is installed](https://golang.org/doc/install)
and that
[`$GOPATH` and `$GOBIN` are set](https://golang.org/doc/code.html#GOPATH).

```bash
go get -u github.com/dedis/cothority/app/cosi
go get -u github.com/dedis/cothority/app/cothorityd
```

The two binaries `cosi` and `cothorityd` will be installed in
the directory indicated by `$GOBIN`.

## Running your own CoSi server

First you need to create a configuration file for the server including a 
public/private key pair for the server. 
You can create a default server configuration with a fresh 
public/private key pair as follows:

```bash
cothorityd setup
```

Follow the instructions on the screen. At the end, you should have two files:
* One local server configuration file which is used by your cothority server,
* One group definition file that you will share with other cothority members and
  clients that wants to contact you.

To run the server, simply type:
```bash
cothorityd
```

The server will try to read the default configuration file; if you have put the
file in a custom location, provide the path using:
```base
cothorityd -config path/file.toml
``` 

### Creating a Collective Signing Group
By running several `cothorityd` instances (and copying the appropriate lines 
of their output) you can create a `servers.toml` that looks like 
this:

```
Description = "My Test group"

[[servers]]
  Addresses = ["127.0.0.1:2000"]
  Public = "6T7FwlCuVixvu7XMI9gRPmyCuqxKk/WUaGbwVvhA+kc="
  Description = "Local Server 1"

[[servers]]
  Addresses = ["127.0.0.1:2001"]
  Public = "Aq0mVAeAvBZBxQPC9EbI8w6he2FHlz83D+Pz+zZTmJI="
  Description = "Description of the server"
```

Your list will look different, as the public keys will not be the same. But
it is important that you run the servers on different ports. Here the ports
are 2000 and 2001.
 
### Checking server-list

The `cosi`-binary has a command to verify the availability for all
servers in a `servers.toml`-file:

```bash
cosi check
```

This will first contact each server individually, then make a small cothority-
group of all possible pairs of servers. If there is a problem with regard to
some firewalls or bad connections, you will see a "Timeout on signing" error
message and you can fix the problem.

### Publicly available DeDiS-CoSi-servers

For the moment there are four publicly available signing-servers, without
any guarantee that they'll be running. But you can try the following:

```bash
cat > servers.toml <<EOF

[[servers]]
  Addresses = ["78.46.227.60:2000"]
  Public = "2juBRFikJLTgZLVp5UV4LBJ2GSQAm8PtBcNZ6ivYZnA="
  Description = "Profeda CoSi server"

[[servers]]
 Addresses = ["5.135.161.91:2000"]
 Public = "jJq4W8KaIFbDu4snOm1TrtrtG79sZK0VCgshkUohycA="
 Description = "Nikkolasg's server"

[[servers]]
  Addresses = ["185.26.156.40:61117"]
  Public = "XEe5N57Ar3gd6uzvZR9ol2XopBlAQl6rKCbPefnWYdI="
  Description = "Ismail's server"

[[servers]]
  Addresses = ["95.143.172.241:62306"]
  Public = "ag5YGeVtw3m7bIGF57X+n1X3qrHxOnpbaWBpEBT4COc="
  Description = "Daeinar's server"
EOF
```

And use the created servers.toml for signing your messages and files.

## Initiating the Collective Signing Protocol

If you have a valid `servers.toml`-file, you can collectively 
sign a text message specified on the command line as follows:

```bash
cosi sign msg "Hello CoSi"
```

cosi will contact the servers and print the signature to the STDOUT. If you
copy that signature to a file called `msg.sig`, you can verify your message
with

```bash
cosi verify msg "Hello CoSi" -sig msg.sig
```

If you would instead like to sign a message contained in a file you 
specify (which may be either text or arbitrary binary data), you can do 
this as follows:

```bash
cosi sign file file-to-be-signed
```

It will create a file `file-to-be-signed.sig` containing the sha256 hash
of the the file and the signature.
To verify the signature of a file you write:
  
```bash
cosi verify file file-to-be-signed
```
    
For all commands, if you chose another filename for the servers than `servers.toml`, you can
give that on the command-line, so for example to sign a message:

```bash
cosi -servers my_servers.toml sign msg "Hello CoSi"
```

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
* [`simul/`](simul/): used for running simulations on localhost and 
DeterLab
