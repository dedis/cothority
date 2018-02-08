# CoSi

This package provides functionality to request and verify collective signatures as well as run a standalone server for handling collective signing requests.

## Getting Started

To use the code of this package you need to:

- Install [Golang](https://golang.org/doc/install)
- Optional: Set [`$GOPATH`](https://golang.org/doc/code.html#GOPATH) to point to your workspace directory
- Put $GOPATH/bin in your PATH: `export PATH=$PATH:$(go env GOPATH)/bin`

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
export COTHORITY=$(go env GOPATH)/src/github.com/dedis/cothority/dedis-cothority.toml 
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

## Description
The purpose of this work is to implement a robust and scalable consensus algorithm using CoSi protocol and handling some exceptions. **The CoSi tree is a three level tree** to make a compromise between the two-level tree, making the root-node vulnerable to DoS, and a more than three level tree, slowing the algorithm because of the RTT between the root node and the leaves.

The tree is composed of a leader (root-node), and some groups of equal size, each having a sub-leader (second level nodes) and members (leaves). The groups composition are defined by the leader.

We want to **handle non-responding nodes**, no matter where they are in the tree. If a leaf is failing, then it is ignored in the CoSi commitment. If a sub-leader is non-responding, then the leader (root node) recreates the group designing another sub-leader from the group members. And finally, if the leader is failing, the protocol restarts using another leader.

More complex adversaries (modifying messages, non-responding at challenge time, etc.) are not yet handled.
The purpose of the project is to **test scalability and robustness** of this service on a testbed and to have a well-documented **reusable code** for it.


## References
- OmniLedger: A Secure, Scale-Out, Decentralized Ledger via Sharding: https://eprint.iacr.org/2017/406.pdf part 4 A & B
- (CoSi) Keeping Authorities "Honest or Bust" with Decentralized Witness Cosigning: https://arxiv.org/abs/1503.08768
- (ByzCoin) Enhancing Bitcoin Security and Performance with Strong Consistency via Collective Signing: https://arxiv.org/abs/1602.06997


## Further Information

For more details, e.g., to learn how you can run your own CoSi server or cothority, see the [wiki](https://github.com/dedis/cothority/wiki/CoSi).
