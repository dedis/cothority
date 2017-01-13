[![Build Status](https://travis-ci.org/dedis/cothority.svg?branch=master)](https://travis-ci.org/dedis/cothority)
[![Coverage Status](https://coveralls.io/repos/github/dedis/cothority/badge.svg?branch=master)](https://coveralls.io/github/dedis/costhority?branch=master)


# CoSi

This package provides functionality to request and verify collective signatures as well as run a standalone server for collective signing.

## Getting Started

To use the code of this package you need to:

-  Install [Golang](https://golang.org/doc/install)
-  Configure your system's [`$GOPATH`](https://golang.org/doc/code.html#GOPATH) variable

To build and install the CoSi application, execute:

```
go get -u github.com/dedis/cothority/cosi
```

## Functionality Overview

```
cosi help
NAME:
   CoSi App - Collectively sign or verify a file; run a server for collective signing

USAGE:
   cosi [global options] command [command options] [arguments...]

VERSION:
   0.10

COMMANDS:
     sign, s    Requests a collectively signature for a 'file'; signature is written to STDOUT by default
     verify, v  Verifies a collective signature of a 'file'; signature is read from STDIN by default
     check, c   Checks if the servers in the group definition are up and running
     server     Starts a CoSi server
     help, h    Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug value, -d value  debug-level: 1 for terse, 5 for maximal (default: 0)
   --help, -h               show help
   --version, -v            print the version
```

## Using the CoSi Client

### Configuration

To tell the CoSi client which existing cothority (public key) it should use for signing requests (signature verification), you need to specify a configuration file. For example, you could use the [DEDIS cothority configuration file](../dedis-cothority.toml) which is included in this repository. To have a shortcut for later on, set:

```
export COTHORITY=$GOPATH/src/github.com/dedis/cothority/dedis-cothority.toml 
```

### Usage

To request a collective (Schnorr) signature `file.sig` on a `file` from the DEDIS cothority, use:

```
cosi sign -g $COTHORITY -o file.sig file
```

To verify a collective (Schnorr) signature `file.sig` of the `file`, use:

```
cosi verify -g $COTHORITY -s file.sig file
```

To check the status of a collective signing group, use:

```
cosi check -g $COTHORITY
```

This will first contact each server individually and then check a few random collective signing group constellations. If there are connectivity problems, due to firewalls or bad connections, for example, you will see a "Timeout on signing" or similar error message.

## Further Information

For more details, e.g., to learn how you can run your own CoSi server or cothority, see the [wiki](https://github.com/dedis/cothority/wiki/CoSi).
