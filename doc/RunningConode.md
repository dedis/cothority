Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Running a Conode

# Running a Collective Authority Node - Conode

Conode is the program that allows you to be part of a cothority. For the server you need:

- 24/7 availability
- 512MB of RAM and 1GB of disk-space
- a public IP-address and two consecutive, open ports
- go1.9 or go1.10 installed and set up according to https://golang.org/doc/install

This document gives an overview of how to run a conode in a virtual server. For
general handling of conodes, including backup-strategies, see
[Operating a Conode](OperatingConode.md).

# Installation

If you run a conode on your server and make it available to
others, they will be able to sign using your server and thus increase the
security of their signature.

To install conode, make sure that
[Go is installed](https://golang.org/doc/install)
and that
[`$GOPATH` is set](https://golang.org/doc/code.html#GOPATH).

The `conode`-binary will be installed in the directory indicated by `$GOPATH/bin`
with the following command:
```
go get -u github.com/cothority/conode
```

# Running your own conode

First you need to create a configuration file for the server including a
public/private key pair for the server.
You can create a default server configuration with a fresh
public/private key pair as follows:

```
conode setup
```

Follow the instructions on the screen. At the end, you should have two files:
* One local server configuration file which is used by your conode,
* One group definition file that you will share with other cothority members and
  clients that want to contact you.

To run the server, simply type:
```
conode
```

The server reads the default configuration file; if you have put the
file in a custom location, you have to provide the path using:
```
conode -config path/file.toml
```

## Using screen

Or if you want to run the server in the background, you can use the `screen`-program:
```
screen -S conode -d -m conode -d 2
```

To enter the screen, type `screen -r cothority`, you can quit it with `<ctrl-a> d`.

## Verifying your server

If everything runs correctly, you can check the configuration with:

```
conode check ~/.config/conode/group.toml
```

# Updating

To update, enter the following command:

```
go get -u github.com/cothority/conode
```

Then you'll have to enter `screen -r conode`, stop it, and launch it again.
A bash-script is on its way ;)

# Creating your own cothority

Now that you have your own running conode, you can use it to create a cothority
by including it with the set of conodes stored in `dedis-cothority.toml`. Simply
concatenate your group.toml with the official, DEDIS-cothority:
```
cat $GOPATH/src/github.com/cothority/conode/dedis-cothority.toml \
	~/.config/conode/group.toml > whole_group.toml
```

Now you can use `whole_group.toml` to refer to your new cothority that includes
the official DEDIS cothority-servers and your server. If you know of other
people who have running conodes, you can add their `group.toml`-files as well
to create a bigger group.

Simply checking if the cothority is up and reachable can be done with:

```
conode check whole_group.toml
```

You can refer to [Collective Signing](../cosi/README.md) to learn how you can
collectively sign a document. Or have a look at any of the other applications
to learn how to use your new conode:
[Applications](Applications.md).
