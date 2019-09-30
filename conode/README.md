<!-- START doctoc.sh generated TOC please keep comment here to allow auto update -->
<!-- DO NOT EDIT THIS SECTION, INSTEAD RE-RUN doctoc.sh TO UPDATE -->
**:book: Table of Contents**

- [Introduction](#introduction)
  - [Server requirements](#server-requirements)
  - [Network communication ](#network-communication-)
- [Running a conode](#running-a-conode)
  - [Environment setup](#environment-setup)
  - [Configuration setup](#configuration-setup)
    - [Option 1: :computer: Configuration setup with the command line](#option-1-computer-configuration-setup-with-the-command-line)
    - [Option 2: :whale: Configuration setup with docker](#option-2-whale-configuration-setup-with-docker)
    - [Post setup instructions](#post-setup-instructions)
  - [Run your conode](#run-your-conode)
    - [Option 1: :computer: Run with the command line](#option-1-computer-run-with-the-command-line)
    - [Option 2: :whale: Run with docker](#option-2-whale-run-with-docker)
    - [Option 3: `run_nodes.sh`](#option-3-run_nodessh)
- [Maintaining a conode](#maintaining-a-conode)
  - [Backups](#backups)
  - [Recovery from a crash](#recovery-from-a-crash)
  - [Roster IPs should be movable](#roster-ips-should-be-movable)
  - [Verifying your server](#verifying-your-server)
  - [Setting up more than one node](#setting-up-more-than-one-node)
  - [Creating Your Cothority](#creating-your-cothority)
  - [Joining the dedis-cothority](#joining-the-dedis-cothority)
  - [Compiling your docker file](#compiling-your-docker-file)
  - [Development version with docker](#development-version-with-docker)
  - [Docker creation](#docker-creation)
- [For the lazy ones: A survival guide to install your server with Ubuntu 18.04](#for-the-lazy-ones-a-survival-guide-to-install-your-server-with-ubuntu-1804)

<!-- END doctoc.sh generated TOC please keep comment here to allow auto update -->


# Introduction

A **Conode** is a Collective Authority Node and is a server in the cothority.
Conodes are linked together to form a cothority. They can run decentralized
protocols, and to offer services to clients.

The conode in this repository includes all protocols and services and can
be run either for local tests or on a public server. The currently running
conodes are available under http://status.dedis.ch.

To operate a Conode, one need to correctly set up a host and run the Conode
program. The following chapters describe the requirements and needed environment
to correctly run the conode program, as well as general instruction on how to
operate it.

Once you have a conode up and running, you can inform us on dedis@epfl.ch and
we will include your conode in the DEDIS-cothority.

## Server requirements

- 24/7 availability
- 512MB of RAM and 1GB of disk-space
- a public IP-address and two open ports

## Network communication 

Two distinct communication schemes need to be configured:

1) **conode-conode** communication, and
2) **Client to conode** communication.

The conode-conode communication happens for consensus-based transactions in
the cothority, for example when the Conodes need to reach a consensus to store a
transaction on the skip-chain. The client-conode communication happens when
something from the outside of cothority performs a query. It happens when
someone wants to store a value on the skip-chain. In this case, the client will
contact the cothority via the client-conode communication scheme.

# Running a conode

## Environment setup

As we will discover later, conode-conode communication is automatically secured
via TLS when you use the configuration from `conode setup` unchanged. However,
conode-client communication happens on the next port up from the conode-conode
port and it defaults to WebSockets inside of HTTP. Therefore, it is highly
recommended to arrange for this port to be wrapped in TLS as well.

The current recommended way to add HTTPS to the WebSocket port is to use a web
server like Apache or Nginx in reverse proxy mode to forward connections from
port 443 to the WebSocket port, which is the conode's port plus 1.

Here is an example config for Apache using a Let's Encrypt certificate:

```apache
<IfModule mod_ssl.c>
<VirtualHost *:443>
   ServerName excellent.example.com

   # If conode is running on port 7000, non-TLS WebSocket is on 7001,
   # so the reverse proxy points there.
   ProxyPass / ws://localhost:7001/

   SSLCertificateFile /etc/letsencrypt/live/excellent.example.com/fullchain.pem
   SSLCertificateKeyFile /etc/letsencrypt/live/excellent.example.com/privkey.pem
   Include /etc/letsencrypt/options-ssl-apache.conf
</VirtualHost>
</IfModule>
```

And here is a version with Nginx:

```nginx
location / {
   server_name example.com;
   # ...
   ssl_certificate /etc/nginx/ssl/example.com.certificate.pem;
   ssl_certificate_key /etc/nginx/ssl/example.com.key.pem;
   # ...
   location /conode/ {
      proxy_http_version 1.1;

      proxy_set_header Upgrade $http_upgrade;
      proxy_set_header Connection "upgrade";		

      proxy_pass "http://localhost:7771/";
   }
}
```

As we will see later during the configuration phase, we will have to advertise
this configuration in the `public.toml` configuration file with the `Url` field.
In this case, the configuration would be `Url ="https://excellent.example.com"`
for Apache and `Url = "example.com/conode"` for Nginx. More on that later.

## Configuration setup

During the setup phase, the conode program creates its public/private key and
prompts the user with some questions:

- PORT: the indicated port and port+1 to be used for communication.
- IP-address: if it cannot detect your IP-address, it will ask for it. This
usually means that something is wrong. Perhaps you didn't allow your firewall
to accept incoming connections.
- Description: any description you want to share with the world.
- Folder: press `<enter>` for the default folder.

Once the interactive setup is done, the program has created two configuration
files:

- The *public configuration file* (public.toml), which holds the public key,
  network information, and a description. This file is the one that should be
  sent to other cothority operators to request access to the cothority.
- The *private configuration file* (private.toml), which holds the server
  config, including the private key and network configuration, like the server's
  public address on the network. The server will listen to this port, as well as
  to this port + 1 (for conode-conode and conode-client connections,
  respectively).

**Warning:** Never (!!!) share the file `private.toml` with anybody, as it
contains the private key of your conode.

There are two options to run the configuration setup: using the command line or
using docker.

### Option 1: :computer: Configuration setup with the command line

First, we need to get the conode program. The recommended way for getting the
conode program is to download it from the official releases on
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

Then, you can launch the interactive setup configuration by running 

```bash
$ conode setup
``` 

Once done, the configuration files (the public.toml and private.toml) are saved
in the default location or in the one specified during the interactive setup.

The default locations are the following depending on the operating system:

- Linux: `$HOME/.config/conode`
- MacOS: `$HOME/Library/Application Support/conode`
- Windows:`%AppData%\Conode`

### Option 2: :whale: Configuration setup with docker

If you have docker installed and running on your host, you can fire the
interactive setup with the following command:

```
$ mkdir ~/conode_data
$ docker run -it --rm -p 7770-7771:7770-7771 --name conode -v ~/conode_data:/conode_data dedis/conode:latest ./conode setup
```

This will prompt the interactive setup and set the 2 configuration files in the
`~/conode_data` directory.

### Post setup instructions

There are a few things you can do now that you have your configuration files.
This first one is to update the `Url` field in the public.toml file in case you
set up a WebSocket over TLS connection for the conode-client communication
during the [Environment setup](#environment-setup).

Another option concerns the conode-conode communication that runs on TLS. If you
would like the conode to run TLS on the WebSocket interface, you can tell it
where to find the private key and a certificate for that key in the
`private.toml` file with the following fields:

```
WebSocketTLSCertificate = "/etc/fullchain.pem"
WebSocketTLSCertificateKey = "/etc/privkey.pem"
```

In this case, it is up to you to get get a certificate from a certificate
authority and to update `fullchain.pem` when needed to renew the certificate.

Using the Let's Encrypt CA and the `certbot` client program, you can get free
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

## Run your conode

Once the setup is done with one of the two options, you can finally run your
conode depending on the previously chosen option.

### Option 1: :computer: Run with the command line

Simply launch the following:

```bash
$ conode server
```

It is recommended to use more verbose logging and this can be done with the `-d`
option. It is also possible to specify a different location than the default one
for the private.toml file with the `-c` option. If you used Docker to set up your
conode, then the private.toml file is not in the default location. You can then
use:

```bash
$ conode -d 2 -c ~/conode_data/private.toml server
```

You can use the `-h` for a description of the available commands.

If you want to run the server in the background, you can use the `screen`
program:

```bash
$ screen -S conode -d -m conode server
```

To enter the screen, type `screen -r conode`. You can detach from it with
`<ctrl-a> d`.

Note: The logs are not saved by default. If you want to keep a trace of the
logs, it is recommended to use `tee`:

```bash
$ conode server | tee logfile.txt
```

### Option 2: :whale: Run with docker

Type the following to start the conode program with docker:

```bash
$ docker run --restart always -d -p 7770-7771:7770-7771 --name conode -v ~/conode_data:/conode_data dedis/conode:latest
```

Because it will run detached, you can use `docker logs -f conode` to see the
logs. It will be restarted on the next boot as well. To stop it, you can use
`docker stop`. Then, you must remove it with `docker rm <id>` (the id is found
with `docker ps -a`).

Pro tip: you can use the following options for docker in order to gracefully
handle the logs and prevent a disk saturation: 
`--log-opt max-size=10m --log-opt max-file=4 --log-opt compress=true`

If you have systemd, you can simply copy the `conode.service` file and add it to
your systemd-startup. Of course, you should do this as a non-root user:

```
$ wget https://raw.githubusercontent.com/dedis/cothority/conode/conode.service
$ systemctl --user enable conode.service
$ systemctl --user start conode
```

Unfortunately, systemd doesn't allow a user to run a service at system startup,
and all user services get stopped once the user logs out!

### Option 3: `run_nodes.sh`

For development purposes, the `run_nodes.sh` script can be used to launch
multiple conodes. For example, the following command:

```bash
$ ./run_nodes -d tmp -v 2 -n 5
```

will run 5 conodes and save their files in the tmp directory with verbosity of
2. The file containing all the public configurations will be in
"tmp/public.toml".

# Maintaining a conode

## Backups

There are two important files to backup: 

- The `private.toml` file containing the
conode's configuration along with its private key, and
- the `<id>.db` file
containing the database, where `<id>` is in fact the sha256 of the public keys.

On linux, the `private.toml` file is located in
`$HOME/.config/conode/private.toml`, and the database file is in
`$HOME/.local/share/conode/<id>.db`.

When using docker, everything is stored in `$HOME/conode_data`.

The DB file is a [BoltDB](https://github.com/etcd-io/bbolt) file, and more
information about considerations while backing them up is in [Database
backup](https://github.com/dedis/onet/tree/master/Database-backup-and-recovery.md).

## Recovery from a crash

If you have a backup of the private.toml file and a recent backup of the .db
file, you can put them onto a new server, and start the conode. The IP address
in the private.toml file must match the IP address on the server.

## Roster IPs should be movable

To facilitate IP address switches, it is recommended that the public IP
address for the leader of critical skipchains should be a virtual address. For
example, if you have two servers:
* 10.0.0.2 conode-live, also with secondary address 10.0.0.1
* 10.0.0.3 conode-standby

You can keep both servers running, and use scp to move the DB file from
conode-live to conode-standby. Both servers should have the same private.toml
file, which includes the line `Address = "tcp://10.0.0.1:7770"`

If conode-live is down and unrecoverable, you can add 10.0.0.1 as a secondary
address to conode-standby and start the conode on it. From this moment on, you
must be sure that conode-live does not boot, or if it does, that it *does not*
have the secondary address on it anymore. You could do so by not adding the
secondary address to boot-time configs and only move it manually.

The address 10.0.0.1 will be in the Roster of any skipchains, and nodes which
are following that skipchain will still be able to contact the leader, even if
it is now running on a different underlying server.

Note: The address part of a server identity has name resolution applied to it.
Thus it would be possible to set the roster of a skipchain to include a server
identity like "tcp://conode-master.example.com:6979" and then change the
definition of conode-master.example.com in DNS to change the IP address
of the master.

## Verifying your server

You can check if the configuration file is correct with:

```bash
conode -d 3 check ~/.config/conode/public.toml
```

## Setting up more than one node

You can start multiple nodes on the same server by using one user per node and
set up the nodes as described above. Be sure to change the port-numbers and
remember that two ports are used.

## Creating Your Cothority

For most of the apps, you need at least 3 running nodes. Once you have them up
and running, you will need a `roster.toml` that includes all the
`public.toml`-files from your conodes:

```
cat ../*/conode_data/public.toml > roster.toml
```

## Joining the dedis-cothority

The only existing cothority for the moment is available at
http://status.dedis.ch. You can send us an email at dedis@epfl.ch to be added to
this list.

## Compiling your docker file

To create your docker-image and use it, you can create it like this:

```bash
$ go get github.com/dedis/cothority
$ cd $(go env GOPATH)/src/github.com/dedis/cothority/conode
$ make docker
```

If you use `make docker_run` the first time, a directory called `conode_data`
will be created and you will be asked for a port - use 7770 or adapt the
Makefile - and a description of your node. Your public and private key for the
conode will be stored in `conode_data`. If you run `make docker_run` again, the
stored configuration will be used.

To stop the docker, simply run `make docker_stop` or kill the docker-container.
All the configuration is stored in `conode_data`

## Development version with docker

For the latest and greatest version of the conode, you can replace
`conode:latest` with `conode:dev` and you should get a stable, but changing
conode. This means, that to use all the functionalities you need to update the
apps and follow the latest `conode:dev` container regularly.

## Docker creation

For creating a new docker image, there are two commands:

* `make docker_dev` - creates a docker image with the currently checked out
  versions on your machine.
* `make docker BUILD_TAG=v3.0.0-pre1` - creates a docker image from source at tag
BUILD_TAG.

# For the lazy ones: A survival guide to install your server with Ubuntu 18.04

In this section, we provide "as-is" instructions to set up a conode server from
scratch on Ubuntu 18.04 witch nginx, letsencrypt, and docker. We assume that you
start with a fresh install and are logged as root.

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
$ mkdir ~/.ssh # if this is a fresh account, the folder might not exist yet
$ vim ~/.ssh/authorized_keys
> *add pub key*
# Allow connecting with the new SSH port
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
$ sudo ufw allow "Nginx Full"
$ sudo ufw status
$ systemctl status nginx
```

**Setup Nginx virtual host**

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

Here is a complete example of an Nginx config (without using letsencrypt):

```nginx
server {
   listen 80 default_server;
   listen [::]:80 default_server;
   server_name example.com www.example.com;
   return 301 "https://example.com${request_uri}";
}

server {
   listen 443 default_server ssl;
   listen [::]:443 default_server ssl;
   ssl on;
   ssl_certificate /etc/nginx/ssl/example.com.certificate.pem;
   ssl_certificate_key /etc/nginx/ssl/example.com.key.pem;

   ssl_protocols TLSv1 TLSv1.1 TLSv1.2;
   ssl_prefer_server_ciphers on;
   ssl_ciphers 'EECDH+AESGCM:EDH+AESGCM:AES256+EECDH:AES256+EDH';

   root /var/www/example.com/public_html/;
   index index.html index.htm index.nginx-debian.html;
   server_name example.com www.example.com;

   location / {
      try_files $uri $uri/ =404;
   }

   location /conode/ {
      proxy_http_version 1.1;

      proxy_set_header Upgrade $http_upgrade;
      proxy_set_header Connection "upgrade";		

      proxy_pass "http://localhost:7771/";
   }
}
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

**Use docker without root**

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
$ vim ~/conode_data/public.toml
> *update the Url field*
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
