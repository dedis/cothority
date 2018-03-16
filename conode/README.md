Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
Conode

# Conode

A Conode is a Collective Authority Node and is a server in the cothority.
Conodes are linked together to form a cothority. They are able to run
decentralized protocols, and to offer services to clients.

The conode in this repository includes all protocols and services and can
be run either for local tests or on a public server. The currently running
conodes are available under http://status.dedis.ch.

You can run the conode either using the binary, the `run_conode.sh`-script
or with docker:

- Using [command line](CLI.md)
- Using [Docker](Docker.md)

## Operating a Conode

Conode is the program that allows you to be part of a cothority. For the server you need:

- 24/7 availability
- 512MB of RAM and 1GB of disk-space
- a public IP-address and two consecutive, open ports
- go1.9 or go1.10 installed and set up according to https://golang.org/doc/install

You find further information about what is important when you operate a conode
in the following document: [Operating a Conode](Operating.md).

Once you have a conode up and running, you can inform us on dedis@epfl.ch and
we will include your conode in the DEDIS-cothority.

## Creating Your own Cothority

For most of the apps you need at least 3 running nodes. Once you have them up
and running, you will need a `roster.toml` that includes all the
`public.toml`-files from your conodes:

```
cat ../*/conode_data/public.toml > roster.toml
```

You will find more details about the available apps on
[Applications](https://github.com/dedis/cothority/tree/master/doc/Applications.md).
