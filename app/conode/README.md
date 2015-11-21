#CoNode

This repository provides a first implementation of a cothority node (CoNode) for
public usage. After setup the CoNode can be used as a public timestamp-server,
which takes a hash and returns the signature together with the inclusion-proof.
Moreover, a simple program is provided that can generate and verify signatures
of given files.

## Limitations / Disclaimer

There are some known limitations that we would like to address as soon as possible:

* There is no exception-handling if a node is down.
* Each time you add nodes to your tree, the collective public signature changes.
* **BACKDOOR-POSSIBILITY:** the script `start-conode` downloads the latest
version of the CoNode-binaries, together with the definition of the tree. This
means that we have to trust GitHub for not putting up something fancy, and you
have to trust us, we don't do it neither.

## Requirements

* A server with a public IP4 address and two open ports (default: `2000` and `2001`).
* [Golang](https://golang.org/) version 1.5.1 or newer, in case you plan to compile CoNode yourself.

## Setup CoNode and Participate in the EPFL-CoNode-Project

The following steps are necessary to setup CoNode and participate in the
EPFL-CoNode-cluster:

1. Getting CoNode
    a) Download binaries
    b) Compile binaries
2. Configuring CoNode
3. Launching CoNode
4. Using CoNode to stamp documents
5. Updating CoNode


## Getting CoNode

There are two options to get CoNode, either download the pre-compiled binaries
or compile CoNode yourself.

### Download Binaries

The latest binaries of CoNode (for 32-/64-bit Linux and OSX) are available at:

https://github.com/dedis/cothority/releases/latest

Execute the following steps to get a basic setup:

```
mkdir conode
cd conode
wget https://github.com/DeDiS/cothority/releases/download/0.5/conode-0.5.7.tar.gz
tar -xvzf conode-0.5.7.tar.gz
```

### Compile Binaries

This assumes that you have a working [Golang](https://golang.org) installation,
version 1.5.1 or newer. To compile CoNode execute the following steps:

```
go get github.com/dedis/cothority
cd $GOPATH/src/github.com/dedis/cothority
git checkout development
go get ./...
cd app/conode
go build
```


## Configuring CoNode

We recommend that you run CoNode inside a terminal multiplexer such as
[GNU screen](https://www.gnu.org/software/screen/) or [tmux](https://tmux.github.io/) to ensure that CoNode remains available even after
you log out of your server.

After you have downloaded or built the CoNode binary, execute

```
./start-conode.sh setup <IP address>:<port>
```

which starts the configuration process and generates a key pair `key.pub` and `key.priv`.

The file `key.pub` contains a *public key* and the *IP address* and *port
number* as specified above. If no port number is given, then the default value
`2000` is used. Please send `key.pub` to dev.dedis@epfl.ch in order to be
included in the EPFL-CoNode-cluster.

The file `key.priv` contains the *secret key* of your CoNode and should **not be
shared** under any circumstances!

For a proper configuration, your CoNode has to be available under the given IP
address and port for at least 24 hours. Afterwards it will exit automatically,
get the tree-information, and switch to running-mode.


**Additional configurations:** TODO: key setup.


## Using CoNode

After your CoNode has been validated and switched to running mode you can use it
for stamping documents. If your CoNode is not running, you can always start it
manually  by simply executing:

```
./start-conode.sh run
```


### Stamping Documents
### Verifying the Signature of a Document

## Updating CoNode
