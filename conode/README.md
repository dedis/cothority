
<!-- START doctoc.sh generated TOC please keep comment here to allow auto update -->
<!-- DO NOT EDIT THIS SECTION, INSTEAD RE-RUN doctoc.sh TO UPDATE -->
**Table of Contents**

- [Introduction](#introduction)
- [Operating a Conode](#operating-a-conode)
  - [Server requirements](#server-requirements)
  - [Network communication ](#network-communication-)
  - [TLS configuration](#tls-configuration)
  - [Built-in TLS: specifying certificate files](#built-in-tls-specifying-certificate-files)
  - [Built-in TLS: Using Let's Encrypt certificates](#built-in-tls-using-lets-encrypt-certificates)
  - [Backups](#backups)
  - [Recovery from a crash](#recovery-from-a-crash)
  - [Roster IPs should be movable](#roster-ips-should-be-movable)
- [Running a conode](#running-a-conode)
  - [Option 1: The command line](#option-1-the-command-line)
    - [Installation](#installation)
    - [Configuration](#configuration)
    - [Running the conode](#running-the-conode)
    - [Using screen](#using-screen)
    - [Verifying your server](#verifying-your-server)
    - [Conode Help](#conode-help)
  - [Option 2: Using docker](#option-2-using-docker)
    - [Starting Conode](#starting-conode)
    - [Using systemd](#using-systemd)
    - [Setting up more than one node](#setting-up-more-than-one-node)
    - [Joining the dedis-cothority](#joining-the-dedis-cothority)
    - [Compiling your own docker file](#compiling-your-own-docker-file)
    - [Apps](#apps)
    - [Development version](#development-version)
  - [Option 3: `run_nodes.sh`](#option-3-run_nodessh)
- [Creating Your own Cothority](#creating-your-own-cothority)
- [Docker creation](#docker-creation)
- [For the lazy ones: Survival guide to install your server with Ubuntu 18.04](#for-the-lazy-ones-survival-guide-to-install-your-server-with-ubuntu-1804)

<!-- END doctoc.sh generated TOC please keep comment here to allow auto update -->


# Introduction

A **Conode** is a Collective Authority Node and is a server in the cothority.
Conodes are linked together to form a cothority. They are able to run
decentralized protocols, and to offer services to clients.

The conode in this repository includes all protocols and services and can
be run either for local tests or on a public server. The currently running
conodes are available under http://status.dedis.ch.

# Operating a Conode

To operate a Conode, one need to correctly set up a host and run the Conode
program. The following chapters describe the requirements and needed
environement to correctly run the conode program.

Once you have a conode up and running, you can inform us on dedis@epfl.ch and
we will include your conode in the DEDIS-cothority.

## Server requirements

- 24/7 availability
- 512MB of RAM and 1GB of disk-space
- a public IP-address and two open ports
- Go 1.12.x installed and set up according to https://golang.org/doc/install

## Network communication 

There are two distinct communication schemes that need to be configured:

1) **conode-conode** communication, and
2) **Client to conode** communication.

The conode-conode communication happens for concensus-based transactions in
the cothority, for example when the Conodes need to reach a concesus to store a
transaction on the skip-chain. The client-conode communication happens when
something from the outside of cothority performs a query. It happens when
someone wants to store a value on the skip-chain. In this case, the client will
contact the cothority via the client-conode communication scheme.

## TLS configuration

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

If you would like the conode to run TLS on the WebSocket interface, you can tell
it where to find the private key and a certificate for that key in the
`private.toml` file:

```
WebSocketTLSCertificate = "/etc/fullchain.pem"
WebSocketTLSCertificateKey = "/etc/privkey.pem"
```

In this case, it is up to you to get get a certificate from a certificate
authority, and to update `fullchain.pem` when needed in order to renew the
certificate.

## Built-in TLS: Using Let's Encrypt certificates

Using the Let's Encrypt CA, and the `certbot` client program, you can get free
certificates for domains which you control. `certbot` writes the files it
creates into `/etc/letsencrypt`.

If the user you use to run the conode has the rights to read from the directory
where Let's Encrypt writes the private key and the current certificate, you can
arrange for conode to share the TLS certificate used by the server as a whole:

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

Runing the conode program from the command line is the fastest way to get your
conode runing. 

### Installation

The recommended way for getting the conode program is to dowload
it from the official releases on
[Github](https://github.com/dedis/cothority/releases) (replace with the latest
version):

```bash
$ wget https://github.com/dedis/cothority/releases/download/v3.1.3/conode-v3.1.3.tar.gz
$ tar -xvf conode-v3.1.3.tar.gz
```

Otherwise, you can build it from the sources if you have the latest version of
go:

```
$ git clone https://github.com/dedis/cothority.git
$ cd conode
$ go install ./conode
```

### Configuration

To configure your conode, you need to *open two consecutive ports* (e.g., 7770 and 7771) on your machine, then execute

```
conode setup
```

and follow the instructions of the dialog. After a successful setup, there
should be two configuration files:

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

```bash
conode server
```

### Using screen

Or if you want to run the server in the background, you can use the `screen`-program:
```
screen -S conode -d -m conode -d 2 server
```

To enter the screen, type `screen -r conode`, you can detach from it with
`<ctrl-a> d`.

### Verifying your server

If everything runs correctly, you can check the configuration with:

```bash
conode -d 3 check ~/.config/conode/public.toml
```

### Conode Help

```bash
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

You need to have docker installed. Then use the following command to setup your
conode in the `~/conode_data`-directory:

```
$ docker run -it --rm -p 7770-7771:7770-7771 --name conode -v ~/conode_data:/conode_data dedis/conode:latest ./conode setup
```

This will create a `conode_data` directory and ask you for the configuration details:
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

```bash
$ docker run --restart always -d -p 7770-7771:7770-7771 --name conode -v ~/conode_data:/conode_data dedis/conode:latest
```

Because it will run detached, you can use `docker logs -f conode` to see the logs.

It will be restarted on the next boot as well.

### Using systemd

If you have systemd, you can simply copy the `conode.service` file and add it to
your systemd-startup. Of course you should do this as a non-root user:

```
$ wget https://raw.githubusercontent.com/dedis/cothority/conode/conode.service
$ systemctl --user enable conode.service
$ systemctl --user start conode
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
$ go get github.com/dedis/cothority
$ cd $(go env GOPATH)/src/github.com/dedis/cothority/conode
$ make docker
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
$ cat ../*/conode_data/public.toml > roster.toml
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

# For the lazy ones: Survival guide to install your server with Ubuntu 18.04

In this section we provide "as-is" instructions to set up a conode server
from scratch on Ubuntu 18.04 witch nginx, letsencrypt, and docker. We make the
assumption that you start with a fresh install and are logged as root.

**Update and set up SSH**

```bash
# Update
$ sudo apt update
$ sudo apt upgrade
# Set up a new user, staying as root is frightening
$ adduser deployer
$ usermod -aG sudo deployer
$ su deployer
$ sudo apt-get install vim
$ sudo vim /etc/ssh/sshd_config
# We update the ssh config to improve security. Here are the things we update:
> Port 44
> PermitRootLogin no
> PasswordAuthentication no
> UsePAM no
# SSH is only allowed with pub/priv key. We must then allow us to use our key:
$ vim ~/.ssh/authorized_keys
> *add pub key*
# Allow to connect with the new SSH port
$ sudo ufw allow 44
$ sudo service ssh restart
```

**Install docker**

Those directly come from the [official guide](https://docs.docker.com/install/linux/docker-ce/ubuntu/#install-using-the-repository).

```bash
$ sudo apt-get install \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg-agent \
    software-properties-common

$ curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
$ sudo apt-key fingerprint 0EBFCD88
> 9DC8 5822 9FC7 DD38 854A  E2D8 8D81 803C 0EBF CD88
$ sudo add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"
$ sudo apt-get update
$ sudo apt-get install docker-ce docker-ce-cli containerd.io
$ sudo docker run hello-world
```

**Install Nginx**

```bash
$ sudo apt install nginx
$ sudo ufw allow "Nginx HTTPS"
$ systemctl status nginx
```

**Set up virtual host**

```bash
# First create the location where the website will be stored:
$ sudo mkdir -p /var/www/example.com/public_html
$ sudo chown -R $USER:$USER /var/www/example.com/public_html/
$ vim /var/www/example.com/public_html/index.html
> *insert what you like*
# Create the virtual host from the default one
$ sudo cp /etc/nginx/site-available/default /etc/nginx/site-available/example.com
$ sudo vim /etc/nginx/site-available/example.com
> *update for the domain and location we created (do not need to setup ssl yet)*
# Disable the default host
$ sudo rm /etc/nginx/sites-enabled/default 
# Activate our new virtual host
$ sudo ln -s /etc/nginx/sites-available/example.com /etc/nginx/sites-enabled/
$ sudo service nginx reload
```

**Install letsencrypt**

```bash
$ sudo add-apt-repository ppa:certbot/certbot
$ sudo apt install python-certbot-nginx
$ sudo certbot --nginx -d example.com
$ sudo certbot renew --dry-run
```

**Setup Nginx for the conode**

```bash
$ sudo vim /etc/nginx/site-available/swisscloud.cothority.net
> *add the following in the main server block:*

	location /conode/ {
		proxy_http_version 1.1;

		proxy_set_header Upgrade $http_upgrade;
		proxy_set_header Connection "upgrade";		

		proxy_pass "http://localhost:7771/";
	}
```

**Use docker without root***

```bash
$ sudo groupadd docker
$ sudo gpasswd -a $USER docker
$ sudo service docker restart
```

**Run the conode**

```bash
$ sudo ufw allow 7770
$ mkdir conode_data
$ docker run -it --rm -p 7770-7771:7770-7771 --name conode -v ~/conode_data:/conode_data dedis/conode:latest ./conode setup
$ docker run --restart always -d -p 7770-7771:7770-7771 --name conode -v ~/conode_data:/conode_data --log-opt max-size=10m --log-opt max-file=4 --log-opt compress=true dedis/conode:latest
```

**Run watchtower**

Watchtower automatically updates the container if it finds a new version.

```bash
$ docker run -d \
    --name watchtower \
    -v /var/run/docker.sock:/var/run/docker.sock \
    containrrr/watchtower
```