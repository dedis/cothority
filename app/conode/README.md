#CoNode

This repository provides a first implementation of a cothority node (CoNode) for
public usage. After setup the CoNode can be used as a public timestamp-server,
which takes a hash and returns the signature together with the inclusion-proof.
Moreover, a simple program is provided that can generate and verify signatures
of given files.

## Warning
**The software provided in this repository is highly experimental and under heavy
development. Do not use it for anything security-critical. Use at your own risk!**

## Limitations / Disclaimer

There are some known limitations that we would like to address as soon as possible:

* There is no exception-handling if a node is down.
* Each time you add nodes to your tree, the collective public signature changes.


## Requirements

* A server with a public IPv4 address and two open ports (default: `2000` and `2001`).
* [Golang](https://golang.org/) version 1.5.1 or newer, in case you plan to compile CoNode yourself.

## Setup CoNode and Participate in the EPFL-CoNode-Project

The following steps are necessary to setup CoNode and participate in the
EPFL-CoNode cluster:

1. Getting CoNode
    a) Download binaries
    b) Compile binaries
2. Configuring CoNode
3. Launching CoNode
4. Using CoNode to stamp documents
5. Updating CoNode

## Getting CoNode

There are two options to get CoNode: either download the pre-compiled binaries
or compile the software yourself.

### Download Binaries

The latest binaries of CoNode (for 32-/64-bit Linux and OSX) are available at:

https://github.com/dedis/cothority/releases/latest

**Note:** The binaries are currently **not signed**.

Execute the following steps to get a basic setup:

```
mkdir conode
cd conode
wget https://github.com/DeDiS/cothority/releases/download/0.5/conode-0.5.7.tar.gz
tar -xvzf conode-0.5.7.tar.gz
```

### Compile Binaries

Compilation of CoNode requires a working [Golang](https://golang.org) installation
of version 1.5.1 or newer. To do so, execute the following steps:

```
go get github.com/dedis/cothority
cd $GOPATH/src/github.com/dedis/cothority
git checkout development
go get ./...
cd app/conode
go build
```


## Basic Functionality of CoNode

The `conode` binary currently supports the following commands:

* `keygen`: to generate a public-private key pair
* `run`: to run the CoNode
* `check`: to check if a new CoNode is available
* `build`: to create a `config.toml`-file



## Configuring CoNode

We recommend that you run CoNode inside a terminal multiplexer such as
[GNU screen](https://www.gnu.org/software/screen/) or [tmux](https://tmux.github.io/) to ensure that CoNode remains available even after
you log out of your server.

After you have downloaded or built the CoNode binary, execute

```
./start-conode.sh setup <IP address>:<port>
```

which starts the configuration process and generates a key pair:

* `key.pub`: contains a *public key* as well as the *IP address* and *port
number* as specified above. If no port number is given, then the default value
`2000` is used. Please send `key.pub` to dev.dedis@epfl.ch in order to be
included in the EPFL CoNode cluster.

* `key.priv`: contains the secret *private key* of your CoNode which should **not be
shared** under any circumstances.

In order to finish configuration successfully, your CoNode has to be available
under the given IP address and port for at least 24 hours. Afterwards it will
exit automatically, get the CoNode tree-information, and switch to running-mode.

**Note:** In case your CoNode is shut down for whatever reason, you can always
manually restart it by simply executing:

```
./start-conode.sh run
```

**Note:** `start-conode.sh` is only a wrapper around the `conode` binary,
i.e. all of the above steps can also be realised by using `conode` directly.

**Additional configurations:** TODO: key setup.

## Using CoNode

After your CoNode has been validated and switched to running mode you can use it
to generate stamps (i.e. multi-signatures) for documents or verify that a
document is valid under a given signature. Both functions can be called via the
`stamp` utility.

To **generate a stamp**, run

```
./stamp sign file
```

where `file` is the document you want to stamp. This generates a signature and
inclusion-proof and writes it to `file.sig`.

To **verify a stamp**, call

```
./stamp check file
```

If `file` is present, its hash-value is verified against the value stored
in `file.sig`, otherwise only the information in `file.sig` is verified.

## Updating CoNode

The `start-conode`-script comes with an auto-updating mechanism: every time
CoNode quits (e.g. when the root-node terminates), it checks on GitHub if a new
version is available and if so, downloads the new archive, extracts it and
re-launches itself.

We are aware that this is a security-risk and promise to not use your server
for anything but running CoNode. This mechanism will be replaced at some point
with a secure variant.

If you want to avoid this auto-updating, use the `conode` binary directly.


## Setup Your Own CoNode Cluster

If you want to create your own tree of CoNodes, you need to use the `conode`
binary directly and **not** the script `start-conode`.

On receipt of a new `key.pub` file from a user requesting to participate in your
CoNode cluster, you can check the availability and affiliation of
the server specified in `key.pub` by calling

```
./conode check key.pub
```

After you checked the availability of all the nodes in your cluster, you can
concatenate the `key.pub` files (don't forget your own!) to build a list of
hosts and pass that to your CoNode-app:

```
cat key-1.pub >> hostlist
...
cat key-n.pub >> hostlist
./conode build hostlist
```

This generates a configuration file `config.toml`, which you then have to
distribute to all your users and ask them to restart their CoNodes with the
updated settings.

**Note:** currently there is no support to automatically trigger a restart of
all CoNodes in case the configuration changes.

Finally, start your CoNode by calling:

```
./conode run
```


## Further Technical Information

### config.toml

The file `config.toml` contains:

* The used suite of cryptographic algorithms (for the moment we stick to AES128+SHA256+Ed25519)
* A list of all hosts
* The aggregate public key of the CoNode
* The tree of all hosts together with the public key of each host

**Note**: the aggregate public key changes from one CoNode-installation to
another and even adding or removing a node will change it.

### file.sig

The signature file `file.sig` of a document `file` contains:

* name:  the name of the file
* hash: a SHA256-based hash
* proof:  the inclusion-proof for your document
* signature: the collective signature

If you want to verify a given signature, you need aggregate public key of the
CoNode cluster found in the configuration file `config.toml`.



## Contact Us

If you are running your own CoNode cluster, we would be very happy to hear from you. Do
not hesitate to contact us at

dev.dedis@epfl.ch



# Cothorities - Further Information

* Decentralizing Authorities into Scalable Strongest-Link Cothorities: [paper](http://arxiv.org/pdf/1503.08768v1.pdf), [slides](http://dedis.cs.yale.edu/dissent/pres/150610-nist-cothorities.pdf)
* Certificate Cothority - Towards Trustworthy Collective CAs: [paper](https://petsymposium.org/2015/papers/syta-cc-hotpets2015.pdf)
