Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
[Skipchain](../skipchain/README.md) ::
SkipChain Manager

# SkipChain Manager - scmgr

Using the skipchain-manager, you can set up, modify and query skipchains.
For an actual application using the skipchains, refer
to [CISC](../cisc/README.md).

The `scmgr` will be running on your local machine and it will communicate with
one or more remote conodes. For it to work, you need the `public.toml` of a
running cothority where you have the right to create a skipchain or add new
blocks.

If you want only to test how it works, you can have the conodes and the `scmgr`
on your local mmachine for testing. Then the simplest way to get up and
running is:

```bash
cd cothority/conode
go build
./run_nodes.sh -n 3
```

This will start three conodes locally and create a new `public.toml` that
you can use with scmgr. In the following examples, we suppose that the
`public.toml` file and the `co1`, `co2`, and `co3` directories are in the current
directory and that you installed `scmgr` using

```bash
go build ../scmgr
```

## Securing the conode by creating a link

By default, everybody is allowed to use your conode to create new skipchains.
This is good for testing setups where your conode is in your private network
and cannot be accessed from the outside. However, if you put your conode on the
internet (and you should), you need to create a link with it first, which will
then remove the possibility for 3rd parties to create a new skipchain on your
conode. In a later step we'll let them access your conode for a restricted set
of tasks.

To link your conode and your client, you need to have access to the
`private.toml` file of your conode.

### Link using private.toml

The `private.toml`-file is usually in `~/.config/conode` for Linux-systems and
in `~/Library/Application\ Support/conode` for MacOSX-systems. However, if you
started a local cothority for testing using `run_nodes.sh`, you should have
three directories, `co1`, `co2` and `co3` and the `private.toml`-file will be
in each of those directories.

So for the testing system, the command is:

```bash
./scmgr link add co1/private.toml
```

This command will create a new private/public keypair for your client and
register it with one of your conodes.

## Creating a new skipchain

Now that you are linked to your conode, you can create a new skipchain on it,
with its only participant being _co1_:

```bash
./scmgr skipchain create -b 10 -he 10 co1/public.toml
```

This will ask the first node in the `public.toml` file to be the leader and to
create a new skipchain with `baseheight = 10` and `maximumheight = 10`.
Refer to the Chainiac-paper on description of those heights. TLDR: the bigger
the _baseheight_, the longer the links between the blocks get. The bigger the
_maximumheight_, the longer the longest link gets.

Once the skipchain is created, `scmgr` will print out the ID of the new
skipchain.

## Following a skipchain

Now that the skipchain is created, you can open up your security a bit and decide
that this skipchain is trustworthy and that all nodes participating in it are
also trustworthy. There are three different levels of trust that you can chose:

* ID - only this very skipchain is allowed to use your node to store new blocks
* Restricted - your conode will also accept new skipchains, as long as no other
than a subset of nodes of the given skipchain participate in it
* Any - your conode will also accept new skipchains, as long as _any_ node of the
given skipchain participates

For our example, we will tell _co2_ and _co3_ to follow the skipchain created
above and to accept any new block added to that skipchain:

```bash
./scmgr link add co2/private.toml
./scmgr link add co3/private.toml
./scmgr follow add single SKIPCHAIN_ID localhost:7772
./scmgr follow add single SKIPCHAIN_ID localhost:7774
```

Where _SKIPCHAIN_ID_ has to be replaced by the ID of the skipchain returned from
the `scmgr skipchain create` command above.
`localhost:7772` and `localhost:7774` are the IP addresses and port numbers of
 _co2_ and _co3_ respectively.

Now you can ask your first conode to extend the conodes that participate in the
skipchain to all conodes:

```bash
./scmgr skipchain block add -roster public.toml SKIPCHAIN_ID
```

Now you have a skipchain that includes all of your testing conodes. To see the block that
you just created, you can use:

```bash
./scmgr skipchain block print SKIPBLOCK_ID
```
