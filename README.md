# Cothority

The code in this repository permits the testing and running of a cothority-system together with some applications. It is split up in deployment, application and protocols. The basic cryptographic code comes from [DeDiS/crypto](https://github.com/DeDiS/crypto).

## Deploy

* DeterLab
* Localhost
* Planned:
    * Docker
    * LXC

## Applications

* Timestamping
* Signing
* Shamir-secret-service:
    * regular signing
    * tree signing
* Planned:
	* Randhound - decentrailzed randomness cothority
    * Vote - doesn't run yet

## Protocols

* Collective signing

# How to Run a Cothority

All applications in `app/*` are stand-alone with the correct configuration. They can be used either with the `localhost`- or the `deterlab`-deployment.

## Localhost
To run a simple signing check on localhost, execute the following commands:

```
$ go get ./...
$ cd deploy
$ go build
$ ./deploy -deploy localhost simulation/sign_single.toml
```

## DeterLab

If you use the `-deploy deterlab` option, then you will have to enter the name of the DeterLab-installation, your username, project- and experiment-name. To make your life as a cothority-developer simpler, there are some flags that are only to be used when deploying to DeterLab:

* `-nobuild`: don't build any of the helpers which is useful if you're working on the main code
* `-build "helper1,helper2"`: only build the helpers, separated by a ",", which speeds up recompiling

### SSH-keys
For convenience, we recommend that you upload a public ssh-key to the DeterLab site. If your ssh-key is protected through a passphrase (which should be the case for security reasons!) we further recommend that you add your private key to your ssh-agent / keychain.

**OSX:**

```
/usr/bin/ssh-add -K ~/.ssh/<your private ssh key>
```

Make sure that you actually use the `ssh-add` program that comes with your OSX installation. Those installed through [homebrew](http://brew.sh/), [MacPorts](https://www.macports.org/) etc. **do not support** the `-K` flag.

**Linux:**
```
TODO.
```



# Applications

## CoNode

You can find more information about CoNode in the corresponding [README](https://github.com/dedis/cothority/app/conode/README.md).

## Timestamping

Sets up servers that listen for client-requests, collects all requests and hands them to a root-node for timestamping.

## Signing

A simple mechanism that is capable of receiving messages and returns their signatures.

## RandHound

Test-implementation of a randomization-protocol based on cothority.

# Protocols

We want to compare different protocols for signing and timestamping uses.

## Collective Signing

This one runs well and is described in a pre-print from Dylan Visher.

## Shamir Signing

A textbook shamir signing for baseline-comparison against the collective signing protocol.


# Further Information

* Decentralizing Authorities into Scalable Strongest-Link Cothorities: [paper](http://arxiv.org/pdf/1503.08768v1.pdf), [slides](http://dedis.cs.yale.edu/dissent/pres/150610-nist-cothorities.pdf)
* Certificate Cothority - Towards Trustworthy Collective CAs: [paper](https://petsymposium.org/2015/papers/syta-cc-hotpets2015.pdf)

