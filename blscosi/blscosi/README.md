Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
[BLS Fault Tolerant Collective Signing](README.md) ::
BlsCoSi CLI

# BLS CoSi CLI

To use the code of this package you need to:

- Install [Golang](https://golang.org/doc/install)
- Optional: Set [`$GOPATH`](https://golang.org/doc/code.html#GOPATH) to point to your workspace directory
- Put $GOPATH/bin in your PATH: `export PATH=$PATH:$(go env GOPATH)/bin`

To build and install the blscosi application, execute:

```
go get -u github.com/dedis/cothority/blscosi
```

## Functionality Overview

```
NAME:
   blscosi - collectively sign or verify a file; run a server for collective signing

USAGE:
   blscosi [global options] command [command options] [arguments...]

VERSION:
   1.00

COMMANDS:
     sign, s    Request a collectively signature for a 'file'; signature is written to STDOUT by default
     verify, v  Verify a collective signature of a 'file'; signature is read from STDIN by default
     check, c   Check if the servers in the group definition are up and running
     server     Start blscosi server
     help, h    Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug value, -d value  debug-level: 1 for terse, 5 for maximal (default: 0)
   --help, -h               show help
   --version, -v            print the version
```

## Using the BlsCoSi Client

### Configuration

To tell the client which existing cothority (public key) it should use for signing requests (signature verification), you need to specify a configuration file. For example, you could use the [DEDIS cothority configuration file](../dedis-cothority.toml) which is included in this repository. To have a shortcut for later on, set:

```
export COTHORITY=$(go env GOPATH)/src/github.com/dedis/cothority/dedis-cothority.toml
```

### Usage

To request a collective signature `file.sig` on a `file` from the DEDIS cothority, use:

```
blscosi sign -g $COTHORITY -o file.sig file
```

To verify a collective signature `file.sig` of the `file`, use:

```
blscosi verify -g $COTHORITY -s file.sig file
```

To check the status of a collective signing group, use:

```
blscosi check -g $COTHORITY
```

This will first contact each server individually and then check a few random collective signing group constellations. If there are connectivity problems, due to firewalls or bad connections, for example, you will see a "Timeout on signing" or similar error message.

## References
- OmniLedger: A Secure, Scale-Out, Decentralized Ledger via Sharding: https://eprint.iacr.org/2017/406.pdf part 4 A & B
- (CoSi) Keeping Authorities "Honest or Bust" with Decentralized Witness Cosigning: https://arxiv.org/abs/1503.08768
- (ByzCoin) Enhancing Bitcoin Security and Performance with Strong Consistency via Collective Signing: https://arxiv.org/abs/1602.06997


## Further Information

For more details, e.g., to learn how you can run your own CoSi server or cothority, see the [wiki](../../conode/README.md).
The same applies to blscosi.
