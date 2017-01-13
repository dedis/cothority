# Conode

This package implements the cothority server. Conodes are linked together to form cothorities, run decentralized protocols, and offer services to clients.

## Getting Started

To use the code of this package you need to:

-  Install [Golang](https://golang.org/doc/install)
-  Set [`$GOPATH`](https://golang.org/doc/code.html#GOPATH) to point to your Go workspace directory 
-  Add `$GOPATH/bin` to `$PATH` 

To build and install the cothority server, execute:

```
go get -u github.com/dedis/cothority/conode
```

## Functionality Overview

```
conode help
NAME:
   conode - run a cothority server

USAGE:
   conode [global options] command [command options] [arguments...]

VERSION:
   1.1

COMMANDS:
     setup, s  Setup server configuration (interactive)
     server    Start cothority server
     check, c  Check if the servers in the group definition are up and running
     help, h   Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --config value, -c value  configuration file of the server (default: "/Users/cosh/Library/conode/private.toml")
   --debug value, -d value   debug-level: 1 for terse, 5 for maximal (default: 0)
   --help, -h                show help
   --version, -v             print the version
```

## Using the Cothority Server

### Configuration

To configure your conode you need to *open two consecutive ports* (e.g., 6879 and 6880) on your machine, then execute

```
conode setup
```

and follow the instructions of the dialog. After a successful setup there should be two configuration files:

- The *public configuration file* of your cothority server is located at `$HOME/.config/conode/public.toml`. Adapt the `description` variable to your liking and send the file to other cothority operators to request access to the cothority. 
- The *private configuration file* of your cothoriy server is located at `$HOME/.config/conode/private.toml`.

**Warning:** Never (!!!) share the file `private.toml` with anybody, as it contains the private key of your conode.

**Note:** 

- The [public configuration file](dedis-cothority.toml) of the DEDIS cothority provides an example of how such a file with multiple conodes usually looks like.
- On macOS the configuration files are located at `$HOME/Library/cosi/{public,private}.toml`.

### Usage

To start your conode with the default (private) configuration file, located at `$HOME/.config/conode/private.toml`, execute:

```
conode
```

## run_conode.sh

If you want to run a conode on a long-term basis, you can use `run_conode.sh`. This brings you:

* migration of data if we change it
* loop around conode if it quits
* automatic updates (-update)
* sending of log-files (-mail)

You can run it with all extras like so:

`./run_conode.sh -mail -update`

It will send an email to DEDIS with the last 200 lines of log-output every time conode restarts.
You can change the MAILADDR-variable at the top of the script to change the address 

## Further Information

For further details on the cothority server, please refer to the [wiki](https://github.com/dedis/cothority/wiki/Conode).
