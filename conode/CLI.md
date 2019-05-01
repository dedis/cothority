Navigation: [DEDIS](https://github.com/dedis/doc/README.md) ::
[Cothority](../README.md) ::
[Conode](README.md) ::
Command Line Interface

# Command Line Interface

This document describes how to run a conode from the command line. This is useful
if you have ssh access to a server or a virtual server. To use the code of this
package you need to:

- Install [Golang](https://golang.org/doc/install) - version 1.12 or later
- Optional: Set [`$GOPATH`](https://golang.org/doc/code.html#GOPATH) to point to your workspace directory
- Put $GOPATH/bin in your PATH: `export PATH=$PATH:$(go env GOPATH)/bin`

To build and install the cothority server, execute:

```
go install ./conode
```

## Configuration

To configure your conode you need to *open two consecutive ports* (e.g., 7770 and 7771) on your machine, then execute

```
conode setup
```

and follow the instructions of the dialog. After a successful setup there should be two configuration files:

- The *public configuration file* holds the public key and a description.
Adapt the `description` variable to your liking and send the file to other cothority operators to request
access to the cothority.
- The *private configuration file* of your cothoriy holds the server config, including the private key. It
also includes the server's public address on the network. The server will listen
to this port, as well as to this port + 1 (for websocket connections).

The setup routine writes the config files into a directory depending on the
operating system:
- Linux: `$HOME/.config/conode`
- MacOS: `$HOME/Library/Application Support/conode`
- Windows:`%AppData%\Conode`

**Warning:** Never (!!!) share the file `private.toml` with anybody, as it contains the private key of
your conode.

## Running the conode

To start your conode with the default configuration file, execute:

```
conode server
```

### Using screen

Or if you want to run the server in the background, you can use the `screen`-program:
```
screen -S conode -d -m conode -d 2 server
```

To enter the screen, type `screen -r conode`, you can quit it with `<ctrl-a> d`.

## Verifying your server

If everything runs correctly, you can check the configuration with:

```
conode -d 3 check ~/.local/share/conode/public.toml
```

### Conode Help

```
NAME:
   conode - run a cothority server

USAGE:
   conode [global options] command [command options] [arguments...]

VERSION:
   3.0.0

COMMANDS:
     setup, s  Setup server configuration (interactive)
     server    Start cothority server
     check, c  Check if the servers in the group definition are up and running
     help, h   Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug value, -d value   debug-level: 1 for terse, 5 for maximal (default: 0)
   --config value, -c value  Configuration file (private.toml) of the server (default: os-specific)
   --help, -h                show help
   --version, -v             print the version
```

## Further Information

For further details on the cothority server, please refer to the [Conode](README.md).
