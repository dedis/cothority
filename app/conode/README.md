#CoNode

This repository provides a first implementation of a cothority node (CoNode) for public usage. After setup, a CoNode can be used as a public timestamp-server, which takes a hash and computes a signature together with an inclusion-proof.  Moreover, a simple stamping-program is provided that can generate and verify signatures of given files through the CoNode.

Currently you can run CoNode either by participating in the EPFL CoNode-project or by setting up your own cluster of CoNodes. Both options are described further below.

## Warning
**The software provided in this repository is highly experimental and under heavy development. Do not use it for anything security-critical. All usage is at your own risk!**

## Limitations / Disclaimer

There are some known limitations that we would like to address as soon as possible:

* There is no exception-handling if a node is down.
* Each time you add nodes to your tree, the collective public signature changes.

## Requirements

* A server with a public IPv4 address and two open ports (default: `2000` and `2001`).
* [Golang](https://golang.org/) version 1.5.1 or newer, in case you plan to compile CoNode yourself.


## Getting CoNode

There are two options to get CoNode: either download the pre-compiled binaries or compile the software yourself.

### Download Binaries

The latest binaries of CoNode (for 32-/64-bit Linux and OSX) are available at:

https://github.com/dedis/cothority/releases/latest

**Note:** The binaries are currently **not signed**.

Execute the following steps to get a basic setup:

```
$ mkdir conode
$ cd conode
$ wget https://github.com/DeDiS/cothority/releases/download/0.5/conode-0.5.7.tar.gz
$ tar -xvzf conode-0.5.7.tar.gz
```

### Compile Binaries

Compilation of CoNode requires a working [Golang](https://golang.org) installation of version 1.5.1 or newer. To do so, execute the following steps:

```
$ go get github.com/dedis/cothority
$ cd $GOPATH/src/github.com/dedis/cothority
$ git checkout development
$ go get ./...
$ cd app/conode
$ go build
```


## Overview of CoNode

The `conode` binary provides all the required functionality to configure and run a CoNode (cluster). To get an overview on the supported commands call:

```
$ ./conode --help

NAME:
   Conode - Run a cothority server and contacts others conodes to form a cothority tree

USAGE:
   ./conode [global options] command [command options] [arguments...]

VERSION:
   0.1.0

AUTHOR(S):
   Linus Gasser <linus.gasser@epfl.ch> nikkolasg <not provided yet>

COMMANDS:
   check, c     Check a host to determine if it is a valid node to get incorporated into the cothority tree.
   build, b	    Build a cothority configuration file needed for the conodes and clients.
   exit, x	    Stop a given conode.
   run, r	    Run this conode inside the cothority tree.
   validate, v	conode will wait in validation mode
   keygen, k	Create a new key pair and binding the public part to your address.
   help, h	    Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug, -d "1"  debug level from 1 (only major operations) to 5 (very noisy text)
   --help, -h       show help
   --version, -v    print the version
```

For more information on how to use the above commands please refer to the following sections.  The script `start-conode.sh`, which can be found in the CoNode archive and repository, is a wrapper around `conode` and automatises certain tasks.

**Note:** Since your CoNode should be permanently available, we recommend that you run the program inside a terminal multiplexer such as [GNU screen](https://www.gnu.org/software/screen/) or [tmux](https://tmux.github.io/).  This ensures that your CoNode remains online even after you log out of your server.

## Configuring CoNode

### Key Generation

The **first step** in configuration is to generate a new key pair:

```
$ ./conode keygen <IP address>:<port>
```

This command generates two files:

* `key.pub`: contains a *public key* as well as the *IP address* and *port number* as specified above. If no port number is given, then the default value `2000` is used.
* `key.priv`: contains the *private key* of your CoNode.

If you are not the operator of a CoNode cluster yourself, then you need to send the public key `key.pub` to your CoNode administrator and ask for inclusion in the tree. The private key `key.priv`, however, **must remain secret** under all circumstances and should not be shared!

### Validation Mode

The **second step** is to bring your CoNode into validation mode:

```
$ ./conode validate
```

Then wait until your CoNode operator has verified your instance and has sent you the configuration file `config.toml` containing information about the other nodes in the cluster.

### Running Mode

The **third step** is finally to bring your CoNode into running mode. Therefore, shutdown your CoNode in validation mode and call:

```
$ ./conode run
```


### All-In-One Setup

For a combination of the above steps you can use the `start-conode.sh` script:

```
$ ./start-conode.sh setup <IP address>:<port>
```

This script generates the keys and starts the validation mode. After your CoNode has been available for a long enough time (usually more than 24 hours) under the specified IP address and port and has been validated by your CoNode operator, it will exit automatically, get the CoNode tree-information, and switch to running-mode.

**Note:** In case your CoNode is shutdown for whatever reason, you can always manually restart it by simply executing:

```
$ ./start-conode.sh run
```

The `start-conode`-script comes with an auto-updating mechanism: every time CoNode quits (e.g. when the root-node terminates), it checks on GitHub if a new version is available and if so, downloads the new archive, extracts it and re-launches itself.

We are aware that this is a security-risk and promise to not use your server for anything but running CoNode. This mechanism will be replaced at some point with a secure variant.

If you want to avoid this auto-updating, use the `conode` binary directly as described above.


## Using CoNode

Once your CoNode is properly configured and in running mode, you can use it to generate stamps (i.e. multi-signatures) for documents or verify that a document is valid under a given signature. Both functions can be called via the `stamp` utility.

To **generate a stamp**, run

```
$ ./stamp sign file
```

where `file` is the document you want to stamp. This generates a signature and inclusion-proof and writes it to `file.sig`.

To **verify a stamp**, call

```
$ ./stamp check file
```

If `file` is present, its hash-value is verified against the value stored in `file.sig`, otherwise only the information in `file.sig` is verified.

## Participate in the EPFL CoNode Cluster

In order to participate in the EPFL CoNode project follow the setup steps as described above using either the `start-conode.sh` script or the `conode` binary directly. Please send your `key.pub` file to dev.dedis@epfl.ch and wait until we have validated your instance. For that make sure that your CoNode is available for at least 24 hours under the IP address and port specified in `key.pub`.  Once we have verified your CoNode, we will send you the configuration file `config.toml`. Copy that to the folder of your `conode` binary, shutdown the validation-mode and restart CoNode in running-mode. Now your CoNode is configured and you can `stamp` files through the EPFL CoNode cluster.

## Setup Your Own CoNode Cluster

If you want to create your own tree of CoNodes, you need to use the `conode` binary directly and **not** the script `start-conode`.

On receipt of a new `key.pub` file from a user requesting to participate in your CoNode cluster, you can check the availability and affiliation of the server specified in `key.pub` by calling

```
$ ./conode check key.pub
```

After you checked the availability of all the nodes in your cluster, you can concatenate the `key.pub` files to build a list of hosts and pass that to your CoNode application:

```
$ cat key-1.pub >> hostlist
  ...
$ cat key-n.pub >> hostlist
$ ./conode build hostlist
```

This generates a configuration file `config.toml`, which you then have to distribute to all your users and ask them to restart their CoNodes with the updated settings.

**Note:** currently there is no way to automatically trigger a restart of all CoNodes in case the configuration changes.

Finally, start your CoNode by calling:

```
$ ./conode run
```

## Further Technical Information

### config.toml

The file `config.toml` contains:

* The used suite of cryptographic algorithms (for the moment we stick to AES128+SHA256+Ed25519)
* A list of all hosts
* The aggregate public key of the CoNode
* The tree of all hosts together with the public key of each host

**Note**: the aggregate public key changes from one CoNode-installation to another and even adding or removing a node will change it.

### file.sig

The signature file `file.sig` of a document `file` contains:

* name: the name of the file
* hash: a SHA256-based hash
* proof: the inclusion-proof for your document
* signature: the collective signature

If you want to verify a given signature, you need aggregate public key of the CoNode cluster found in the configuration file `config.toml`.

## Contact Us

If you are running your own CoNode cluster, we would be very happy to hear from you. Do not hesitate to contact us at

dev.dedis@epfl.ch
