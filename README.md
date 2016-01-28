# Cothority

This repository provides a framework for implementing secure, distributed systems. It does so by offering services to run different types of protocols which may rely on other, pre-defined protocols.
 
Using the SDA-cothority framework, you can easily
* Simulate up to 8192 nodes using Deterlab (which is based on Planetlab)
* Run local simulations for up to 128 nodes (restricted by your computer)
* Distribute binaries for real-world deployment

The framework is round-based using message-passing between different hosts which form a tree. Every protocol defines the steps needed to accomplish the calculations, and the framework makes sure that all messages are passed between the hosts.
  
The directory-structure is as follows:
* /lib - holding all internally used libraries
* /lib/sda - the basic definition of our framework
* /protocols - one directory per protocol, holds both the definition and eventual initialisation needed for simulation
* /simul - used for running simulations on localhost and Deterlab
* /dist - creates distributable binaries, in .tgz or Docker-format

## Warning
**The software provided in this repository is highly experimental and under heavy development. Do not use it for anything security-critical. All usage is at your own risk!**

## Further Information

* Decentralizing Authorities into Scalable Strongest-Link Cothorities: [paper](http://arxiv.org/abs/1503.08768), [slides](http://dedis.cs.yale.edu/dissent/pres/151009-stanford-cothorities.pdf)
* Certificate Cothority - Towards Trustworthy Collective CAs: [paper](https://petsymposium.org/2015/papers/syta-cc-hotpets2015.pdf)
* For questions and discussions please refer to our [mailing list](https://groups.google.com/forum/#!forum/cothority).

## Requirements

* Golang 1.5.2+
* [DeDiS/crypto](https://github.com/DeDiS/crypto)

## Simulation

* Available:
    * [DeterLab](http://deterlab.net)
    * Localhost

## Distribution
* Planned:
    * Binary .tar.gz
    * Docker

## Protocols available
The following protocols will be available shortly:
* JVSS - Joint Verifiable Secret Sharing using Shamir's protocol
* RandHound - Creating strong random-numbers
* CoSi - Collective Signing

# Simulation
It is very easy to start a simulation of your protocol either on localhost or, if you have access, on Deterlab.

## Localhost
To run a simple signing check on localhost, execute the following commands:

```
$ go get ./...
$ cd simul
$ go build
$ ./simul runfiles/sign_single.toml
```

## DeterLab

If you use the `-platform deterlab` option, then you are prompted to enter the name of the DeterLab installation, your username, and the names of project and experiment. There are some flags which make your life as a cothority developer simpler when deploying to DeterLab:

* `-nobuild`: don't build any of the helpers which is useful if you're working on the main code
* `-build "helper1,helper2"`: only build the helpers, separated by a ",", which speeds up recompiling
* `-range start:end`: runs only the simulation-lines including `start` and `end`. Counts from 0, start and end can be omitted and represent beginning and end of lines, respectively.

### SSH-keys
For convenience, we recommend that you upload a public SSH-key to the DeterLab site. If your SSH-key is protected through a passphrase (which should be the case for security reasons!) we further recommend that you add your private key to your SSH-agent / keychain. Afterwards you only need to unlock your SSH-agent / keychain once (per session) and can access all your stored keys without typing the passphrase each time.

**OSX:**

You can store your SSH-key directly in the OSX-keychain by executing:

```
$ /usr/bin/ssh-add -K ~/.ssh/<your private ssh key>
```

Make sure that you actually use the `ssh-add` program that comes with your OSX installation, since those installed through [homebrew](http://brew.sh/), [MacPorts](https://www.macports.org/) etc. **do not support** the `-K` flag per default.

**Linux:**

Make sure that the `ssh-agent` is running. Afterwards you can add your SSH-key via:

```
$ ssh-add ~/.ssh/<your private ssh key>
```

# Protocol details

## CoSi - Collective Signing

The system is based on CoSi, a novel protocol for collective signing which itself builds upon Merkle trees and Schnorr multi-signatures over elliptic curves. CoSi enables authorities to have their statements collectively signed (co-signed) by a diverse, decentralized, and scalable group of (potentially thousands of) witnesses and, for example, could be employed to proactively harden critical Internet authorities. Among other things, one could imagine applications to the Certificate Transparency project, DNSSEC, software distribution, the Tor anonymity network or cryptocurrencies.

## JVSS - Shamir Signing

A textbook shamir signing for baseline-comparison against the collective signing protocol.


# Applications

## CoNode

You can find more information about CoNode in the corresponding [README](https://github.com/DeDiS/cothority/blob/development/app/conode/README.md).

## Timestamping

Sets up servers that listen for client-requests, collects all requests and hands them to a root-node for timestamping.

## Signing

A simple mechanism that is capable of receiving messages and returning their signatures.

## RandHound

Test-implementation of a randomization-protocol based on cothority.

