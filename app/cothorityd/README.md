# Cothorityd

This is the cothority-server for creating a node that can be
used in any cothority. It loads automatically all protocols
and services and can be used through the apps here or in
https://github.com/dedis/cosi .

## Installation

To install cothorityd, make sure that
[Go is installed](https://golang.org/doc/install)
and that
[`$GOPATH` is set](https://golang.org/doc/code.html#GOPATH).

```bash
go get -u github.com/dedis/cothority/app/cothorityd
```

The `cothorityd`-binary will be installed in
the directory indicated by `$GOPATH/bin`.

## Running your own cothority-node

First you need to create a configuration file for the server including a 
public/private key pair for the server. 
You can create a default server configuration with a fresh 
public/private key pair as follows:

```bash
cothorityd setup
```

Follow the instructions on the screen. At the end, you should have two files:
* One local server configuration file which is used by your cothority server,
* One group definition file that you will share with other cothority members and
  clients that wants to contact you.

To run the server, simply type:
```bash
cothorityd
```

The server will try to read the default configuration file; if you have put the
file in a custom location, provide the path using:
```base
cothorityd -config path/file.toml
``` 

### Creating a cothority
By running several `cothorityd` instances (and copying the appropriate lines 
of their output) you can create a `servers.toml` that looks like 
this:

```
Description = "My Test group"

[[servers]]
  Addresses = ["127.0.0.1:2000"]
  Public = "6T7FwlCuVixvu7XMI9gRPmyCuqxKk/WUaGbwVvhA+kc="
  Description = "Local Server 1"

[[servers]]
  Addresses = ["127.0.0.1:2001"]
  Public = "Aq0mVAeAvBZBxQPC9EbI8w6he2FHlz83D+Pz+zZTmJI="
  Description = "Description of the server"
```

Your list will look different, as the public keys will not be the same. But
it is important that you run the servers on different ports. Here the ports
are 2000 and 2001.
