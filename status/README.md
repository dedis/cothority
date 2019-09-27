Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
Status

# Status

Status takes in a file containing a list of servers and returns the status
reports of all of the servers.
A status is a list of connections and packets sent and received for each server
in the file.

## Installation

To install the status-binary, enter

```
go get github.com/dedis/cothority/status
```

And then you can run

```
status -g group.toml
```

Where `group.toml` is a list of servers to connect and return
the status on.

## Check connectivity

The status server can also check the connectivity of a roster. This will ask all nodes
in a roster to connect to each other. If one of the connection fails, the service
can try to create a maximal set of nodes that still can communicate with each other.

```
status connectivity group.toml private.toml
```

Possible flags are:
- `-findFaulty` - if the roster cannot communicate, finds the largest set of nodes that can
- `-timeout=duration` - sets the timeout the service waits for the nodes to respond. In case
of `-findFaulty`, that timeout is multiplied by the number of nodes - 1

## Links

- [Client API](service/README.md)
