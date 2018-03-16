Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Conode](README.md) ::
Docker

# Docker

You need a server with a public IP address and at least 1GB of RAM and docker
installed. First you need to setup the conode, use the following command to
setup conode in your `~/conode_data`-directory:

```
docker run -it --rm -p 6879-6880:6879-6880 --name conode -v ~/conode_data:/root/.local/share/conode/ \
          	    -v ~/conode_data:/root/.config/conode/ dedis/conode:latest
```

This will create a `conode_data`-directory and ask you for the configuration details:
- PORT: the indicated port and port+1 will be used for communication
- IP-address: if it cannot detect your IP-address, it will ask for it. This
usually means that something is wrong. Perhaps you didn't allow your firewall
to accept incoming connections
- Description: any description you want to share with the world
- Folder: press <enter> for the default folder - it will be redirected to `conode_data`

There are two important files in there:
- private.toml - do not give this away - it's your private key!
- public.toml - the description of your conode that you can send to dedis@epfl.ch
and ask us to include it

If you change the port-number, you will have to adjust the numbers
used in the `docker run`-command.

### Starting Conode Using Crontab

An easy way to start a conode upon system-startup is crontab. Add the following
line to your crontab (`crontab -e`) and your conode will start with the next
system-startup:

```
@reboot docker run -it --rm -p 6879-6880:6879-6880 --name conode -v ~/conode_data:/root/.local/share/conode/ \
          	    -v ~/conode_data:/root/.config/conode/ dedis/conode:latest
```

### Starting conode using systemd

If you have systemd, you can simply copy the `conode.service`-file and add it to
your systemd-startup. Of course you should do this as a non-root user:

```
wget https://raw.githubusercontent.com/dedis/cothority/docker_conode/conode/conode.service
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

### Apps

For most of the apps you need at least 3 running nodes. Once you have them up
and running, you will need a `roster.toml` that includes all the
`public.toml`-files from your conodes:

```
cat ../*/conode_data/public.toml > roster.toml
```

You will find more details about the available apps on
[Applications](../doc/Applications.md).
