[![Build Status](https://travis-ci.org/cothority/conode.svg?branch=master)](https://travis-ci.org/cothority/conode)
[![Coverage Status](https://coveralls.io/repos/github/cothority/conode/badge.svg)](https://coveralls.io/github/cothority/conode)

# Conode

A conode is a node of a collective authority (cothority) and participates
in protocols that do collective signing, blockchains, password hashing and
many more things. 

You can run your own conode, and/or use the applications provided in this
repository. Either way, be sure to contact us and tell us about your
experience.

## Applications

A list of all applications can be found in the wiki at
[Applications](https://github.com/cothority/conode/wiki/Apps). Here we present
our main application:

### Collective Signing

This application collectively signs a document by a group of conodes that is
called a cothority. To use it, first build the application:
```bash
go get -u github.com/cothority/conode/cosi
```

Now you can sign a document by a cothority. A group of active conodes can be
found in the `dedis-cothority.toml`-file. For shorter examples, we suppose you
define the following variable first:

```bash
COTHORITY=$GOPATH/src/github.com/cothority/conode/dedis-cothority.toml 
```

To sign your document using that cothority, use the following command:

```bash
cosi sign -g $COTHORITY -o your_file.sig your_file
```

Replace `your_file` with a file you want to have signed. Now `your_file.sig`
contains a collective signature of all the conodes from the DEDIS-lab. To
verify the signature, type:

```bash
cosi verify -g $COTHORITY -s your_file.sig your_file
```

## Installation

If you run a conode on your server and make it available to
others, they will be able to sign using your server and thus increase the
security of their signature.

To install conode, make sure that
[Go is installed](https://golang.org/doc/install)
and that
[`$GOPATH` is set](https://golang.org/doc/code.html#GOPATH).

The `conode`-binary will be installed in the directory indicated by `$GOPATH/bin`
with the following command:
```bash
go get -u github.com/cothority/conode
```

### Running your own conode

First you need to create a configuration file for the server including a 
public/private key pair for the server. 
You can create a default server configuration with a fresh 
public/private key pair as follows:

```bash
conode setup
```

Follow the instructions on the screen. At the end, you should have two files:
* One local server configuration file which is used by your conode,
* One group definition file that you will share with other cothority members and
  clients that want to contact you.

To run the server, simply type:
```bash
conode
```

The server reads the default configuration file; if you have put the
file in a custom location, you have to provide the path using:
```bash
conode -config path/file.toml
```

### Using your conode

There are different apps available to integrate your conode in an existing
cothority. The list is at:
[Applications](https://github.com/cothority/conode/wiki/Apps)

# Documentation

Each directory of the conode-repo is a protocol, a service or an app containing
and using other services. You can find more information about the different
protocols, services and apps on our wiki:
[Conode-Wiki](https://github.com/cothority/conode/wiki)

## Linked documentation

Be sure also to check out the following documentation of the other parts of
the project:

- To run and use a conode, have a look at 
	[Cothority Node](https://github.com/cothority/conode/wiki)
	with examples of protocols, services and apps
- To start a new project by developing and integrating a new protocol, have a look at
	the [Cothority Template](https://github.com/cothority/template/wiki)
- To participate as a core-developer, go to 
	[Cothority Network Library](https://github.com/cothority/conet/wiki)

# License

All repositories in https://github.com/cothority are double-licensed under a 
GNU/AGPL 3.0 and a commercial license. If you want to have more information, 
contact us at bryan.ford@epfl.ch or linus.gassser@epfl.ch.

## Contributing

If you want to contribute to Cothority-ONet, please have a look at 
[CONTRIBUTION](https://github.com/cothority/conode/blobl/master/CONTRIBUTION) for
licensing details. Once you are OK with those, you can have a look at our
coding-guidelines in
[Coding](https://github.com/dedis/Coding). In short, we use the github-issues
to communicate and pull-requests to do code-review. Travis makes sure that
everything goes smoothly. And we'd like to have good code-coverage.

# Contact

You can contact us at https://groups.google.com/forum/#!forum/cothority