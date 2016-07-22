# Description

Status takes in a file containing a list of servers and returns the status reports of all of the servers.
A status is a list of connections and packets sent and received for each server in the file.

# Installation

To install the status-binary, enter

```
go get github.com/dedis/cothority/app/status
```

And then you can run

```
status -g group.toml
```

Where `group.toml` is a list of servers to connect and return
the status on.
