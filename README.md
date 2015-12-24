# Cothority

This repository provides an implementation for the prototype of the collective authority (cothority) framework. The system is based on CoSi, a novel protocol for collective signing which itself builds upon Merkle trees and Schnorr multi-signatures over elliptic curves. CoSi enables authorities to have their statements collectively signed (co-signed) by a diverse, decentralized, and scalable group of (potentially thousands of) witnesses and, for example, could be employed to proactively harden critical Internet authorities. Among other things, one could imagine applications to the Certificate Transparency project, DNSSEC, software distribution, the Tor anonymity network or cryptocurrencies.

## Warning
**The software provided in this repository is highly experimental and under heavy development. Do not use it for anything security-critical. All usage is at your own risk!**

## Further Information

* Decentralizing Authorities into Scalable Strongest-Link Cothorities: [paper](http://arxiv.org/pdf/1503.08768v1.pdf), [slides](http://dedis.cs.yale.edu/dissent/pres/151009-stanford-cothorities.pdf)
* Certificate Cothority - Towards Trustworthy Collective CAs: [paper](https://petsymposium.org/2015/papers/syta-cc-hotpets2015.pdf)
* For questions and discussions please refer to our [mailing list](https://groups.google.com/forum/#!forum/cothority).

## Requirements

* Golang 1.5.2+
* [DeDiS/crypto](https://github.com/DeDiS/crypto)

## Deploy

* Available:
    * [DeterLab](deterlab.net)
    * Localhost
* Planned:
    * Docker
    * LXC

## Applications

* Available:
    * Timestamping
    * Signing
    * Shamir-secret-service: regular or tree signing
* Planned:
	* Randhound: decentrailzed randomness cothority
    * Vote

## Protocols

* CoSi: collective signing

# How to Run a Cothority

All applications in `app/*` are stand-alone. Currently, they can be used by deploying to either localhost or DeterLab.

## Localhost
To run a simple signing check on localhost, execute the following commands:

```
$ go get ./...
$ cd deploy
$ go build
$ ./deploy -deploy localhost simulation/sign_single.toml
```

## DeterLab

If you use the `-deploy deterlab` option, then you are prompted to enter the name of the DeterLab installation, your username, and the names of project and experiment. There are some flags which make your life as a cothority developer simpler when deploying to DeterLab:

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

# Applications

## CoNode

You can find more information about CoNode in the corresponding [README](https://github.com/DeDiS/cothority/blob/development/app/conode/README.md).

## Timestamping

Sets up servers that listen for client-requests, collects all requests and hands them to a root-node for timestamping.

## Signing

A simple mechanism that is capable of receiving messages and returning their signatures.

## RandHound

Test-implementation of a randomization-protocol based on cothority.

# Protocols

We want to compare different protocols for signing and timestamping uses.

## Collective Signing

This one runs well and is described in a pre-print from Dylan Visher.

## Shamir Signing

A textbook shamir signing for baseline-comparison against the collective signing protocol.



