Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Conode

- [Introduction](#introduction)
- [Operating a Conode](#operating-a-conode)
  * [Server requirements](#server-requirements)
  * [WebSocket TLS](#websocket-tls)
  * [Reverse proxy](#reverse-proxy)
  * [Built-in TLS: specifying certificate files](#built-in-tls--specifying-certificate-files)
  * [Built-in TLS: Using Let's Encrypt certificates](#built-in-tls--using-let-s-encrypt-certificates)
  * [Backups](#backups)
  * [Recovery from a crash](#recovery-from-a-crash)
  * [Roster IPs should be movable](#roster-ips-should-be-movable)
- [Running a conode](#running-a-conode)
  * [Option 1: The command line](#option-1--the-command-line)
    + [Configuration](#configuration)
    + [Running the conode](#running-the-conode)
    + [Using screen](#using-screen)
  * [Verifying your server](#verifying-your-server)
    + [Conode Help](#conode-help)
  * [Option 2: Using docker](#option-2--using-docker)
  * [Starting Conode](#starting-conode)
    + [Using systemd](#using-systemd)
  * [Setting up more than one node](#setting-up-more-than-one-node)
  * [Joining the dedis-cothority](#joining-the-dedis-cothority)
  * [Compiling your own docker file](#compiling-your-own-docker-file)
  * [Apps](#apps)
  * [Development version](#development-version)
  * [Appendix A: Install your server with Ubuntu 18.04](#appendix-a--install-your-server-with-ubuntu-1804)
- [Creating Your own Cothority](#creating-your-own-cothority)
- [Docker creation](#docker-creation)

<small><i><a href='http://ecotrust-canada.github.io/markdown-toc/'>Table of contents generated with markdown-toc</a></i></small>

# Introduction

A **Conode** is a Collective Authority Node and is a server in the cothority.
Conodes are linked together to form a cothority. They are able to run
decentralized protocols, and to offer services to clients.

The conode in this repository includes all protocols and services and can
be run either for local tests or on a public server. The currently running
conodes are available under http://status.dedis.ch.

# Operating a Conode

Conode is the program that allows you to be part of a cothority. For the server you need:

## Server requirements

- 24/7 availability
- 512MB of RAM and 1GB of disk-space
- a public IP-address and two consecutive, open ports
- Go 1.12.x installed and set up according to https://golang.org/doc/install

You find further information about what is important when you operate a conode
in the following document: [Operating a Conode](Operating.md).

Once you have a conode up and running, you can inform us on dedis@epfl.ch and
we will include your conode in the DEDIS-cothority.

## WebSocket TLS

Conode-conode communication is automatically secured via TLS when you use
the configuration from `conode setup` unchanged.

However, conode-client communication happens on the next port up from the
conode-conode port, and it defaults to WebSockets inside of HTTP. It is
recommended to arrange for this port to be wrapped in TLS as well.

When this port is using TLS, you must explicitly advertise this fact
when you add your server to a cothority. You do this by setting the
Url field in the toml file:

```
[[servers]]
  Address = "tls://excellent.example.com:7770"
  Url = "https://excellent.example.com:7771"
  Suite = "Ed25519"
  Public = "ad91a87dd89d31e4fc77ee04f1fc684bb6697bcef96720b84422437ff00b79e3"
  Description = "My excellent example server."
  [servers.Services]
    [servers.Services.ByzCoin]
      ...etc...
```

## Reverse proxy

Conode should only be run as a non-root user.

The current recommended way to add HTTPS to the websocket port is to use a web
server like Apache or nginx in reverse proxy mode to forward connections from
port 443 to the websocket port, which is the conode's port plus 1.

An example config, for Apache using a Let's Encrypt certificate:

```
<IfModule mod_ssl.c>
<VirtualHost *:443>
        ServerName excellent.example.com
		
		# If conode is running on port 7000, non-TLS websocket is on 7001,
		# so the reverse proxy points there.
        ProxyPass / ws://localhost:7001/
		
		SSLCertificateFile /etc/letsencrypt/live/excellent.example.com/fullchain.pem
		SSLCertificateKeyFile /etc/letsencrypt/live/excellent.example.com/privkey.pem
		Include /etc/letsencrypt/options-ssl-apache.conf
</VirtualHost>
</IfModule>
```

In this case, the Url in the TOML file would be `https://excellent.example.com`
(no port number because 443 is the default for HTTPS).

## Built-in TLS: specifying certificate files

If you would like the conode to run TLS on the WebSocket interface, you
can tell it where to find the private key and a certificate for that
key:

```
WebSocketTLSCertificate = "/etc/fullchain.pem"
WebSocketTLSCertificateKey = "/etc/privkey.pem"
```

In this case, it is up to you to get get a certificate from a
certificate authority, and to update `fullchain.pem` when
needed in order to renew the certificate.

## Built-in TLS: Using Let's Encrypt certificates

Using the Let's Encrypt CA, and the `certbot` client program,
you can get free certificates for domains which you control.
`certbot` writes the files it creates into `/etc/letsencrypt`.

If the user you use to run the conode has the rights to read from the
directory where Let's Encrypt writes the private key and the current
certificate, you can arrange for conode to share the TLS certificate
used by the server as a whole:

```
WebSocketTLSCertificate = "/etc/letsencrypt/live/conode.example.com/fullchain.pem"
WebSocketTLSCertificateKey = "/etc/letsencrypt/live/conode.example.com/privkey.pem"
```

Let's Encrypt certificates expire every 90 days, so you will need
to restart your conode when the `fullchain.pem` file is refreshed.

## Backups

On Linux, the following files need to be backed up:
1. `$HOME/.config/conode/private.toml`
2. `$HOME/.local/share/conode/$PUBLIC_KEY.db`

The DB file is a [BoltDB](https://github.com/etcd-io/bbolt) file, and more
information about considerations while backing them up is in [Database
backup](https://github.com/dedis/onet/tree/master/Database-backup-and-recovery.md).

## Recovery from a crash

If you have a backup of the private.toml file and a recent backup of the .db
file, you can put them onto a new server, and start the conode. The IP address
in the private.toml file must match the IP address on the server.

## Roster IPs should be movable

In order to facilitate IP address switches, it is recommended that the public IP
address for the leader of critical skipchains should be a virtual address. For
example, if you have two servers:
* 10.0.0.2 conode-live, also with secondary address 10.0.0.1
* 10.0.0.3 conode-standby

You can keep both servers running, and use scp to move the DB file from
conode-live to conode-standby. Both servers should have the same private.toml
file, which includes the line `Address = "tcp://10.0.0.1:7770"`

In the event that conode-live is down and unrecoverable, you can add 10.0.0.1 as
a secondary address to conode-standby and start the conode on it. From this
moment on, you must be sure that conode-live does not boot, or if it does, that
it *does not* have the secondary address on it anymore. You could do so by not
adding the secondary address to boot-time configs, and only move it manually.

The address 10.0.0.1 will be in the Roster of any skipchains, and nodes which
are following that skipchain will still be able to contact the leader, even if
it is now running on a different underlying server.

Note: The address part of a server identity has name resolution applied to it.
Thus it would be possible to set the roster of a skipchain to include a server
identity like "tcp://conode-master.example.com:6979" and then change the
definition of conode-master.example.com in DNS in order to change the IP address
of the master.


# Running a conode

## Option 1: The command line

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

### Configuration

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

### Running the conode

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

## Option 2: Using docker

You need a server with a public IP address and at least 1GB of RAM and docker
installed. First you need to setup the conode, use the following command to
setup conode in your `~/conode_data`-directory:

```
docker run -it --rm -p 7770-7771:7770-7771 --name conode -v ~/conode_data:/conode_data dedis/conode:latest ./conode setup
```

This will create a `conode_data`-directory and ask you for the configuration details:
- PORT: the indicated port and port+1 will be used for communication. If you
change this port, also update the ports in the docker-command.
- IP-address: if it cannot detect your IP-address, it will ask for it. This
usually means that something is wrong. Perhaps you didn't allow your firewall
to accept incoming connections
- Description: any description you want to share with the world
- Folder: press <enter> for the default folder - it will be redirected to `conode_data`

There are two important files in there:
- `private.toml` - do not give this away - it's your private key!
- `public.toml` - the description of your conode that you can send to dedis@epfl.ch
and ask us to include it

If you change the port-number, you will have to adjust the numbers
used in the `docker run`-command.

### Starting Conode

Once a conode is setup, you can start it like that:

```
docker run --restart always -d -p 7770-7771:7770-7771 --name conode -v ~/conode_data:/conode_data dedis/conode:latest
```

Because it will run detached, you can use `docker logs -f conode` to see the logs.

It will be restarted on the next boot as well.

#### Using systemd

If you have systemd, you can simply copy the `conode.service`-file and add it to
your systemd-startup. Of course you should do this as a non-root user:

```
wget https://raw.githubusercontent.com/dedis/cothority/conode/conode.service
systemctl --user enable conode.service
systemctl --user start conode
```

Unfortunately systemd doesn't allow a user to run a service at system startup,
and all user services get stopped once the user logs out!

### Setting up more than one node

You can start multiple nodes on the same server by using one user per node and
set up the nodes as described above. Be sure to change the port-numbers and
remember that two ports are used.

### Joining the dedis-cothority

The only existing cothority for the moment is available at
http://status.dedis.ch. You can send us an email at dedis@epfl.ch to be added to
this list.

### Compiling your own docker file

To create your own docker-image and use it, you can create it like this:

```bash
go get github.com/dedis/cothority
cd $(go env GOPATH)/src/github.com/dedis/cothority/conode
make docker
```

If you use `make docker_run` the first time, a directory called `conode_data` will be
created and you will be asked for a port - use 7770 or adapt the Makefile - and a
description of you node. Your public and private key for the conode will be stored
in `conode_data`. If you run `make docker_run` again, the stored configuration will
be used.

To stop the docker, simply run `make docker_stop` or kill the docker-container. All
configuration is stored in `conode_data`

### Apps

For most of the apps you need at least 3 running nodes. Once you have them up
and running, you will need a `roster.toml` that includes all the
`public.toml`-files from your conodes:

```
cat ../*/conode_data/public.toml > roster.toml
```

You will find more details about the available apps on
[Applications](https://github.com/dedis/cothority/tree/master/doc/Applications.md).

### Development version

For the latest and greatest version of the conode, you can replace `conode:latest`
with `conode:dev` and you should get a stable, but changing conode. This means, that
to use all the functionalities you need to update the apps and follow the latest
`conode:dev` container regularly.

## Option 3: `run_nodes.sh`

blabla...

## Appendix A: Install your server with Ubuntu 18.04

# Creating Your own Cothority

For most of the apps you need at least 3 running nodes. Once you have them up
and running, you will need a `roster.toml` that includes all the
`public.toml`-files from your conodes:

```
cat ../*/conode_data/public.toml > roster.toml
```

You will find more details about the available apps on
[Applications](https://github.com/dedis/cothority/tree/master/doc/Applications.md).

# Docker creation

For creating a new docker image, there are two commands:

* `make docker_dev` - creates a docker image with the currently checked out versions
on your machine.
* `make docker BUILD_TAG=v3.0.0-pre1` - creates a docker image from source at tag
BUILD_TAG.
