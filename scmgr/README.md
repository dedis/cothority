# SkipChain Manager - scmgr

Using the skipchain-manager, you can set up, modify and query skipchains.
For an actual application using the skipchains, refer
to [https://github.com/dedis/cothority/cisc].

For it to work, you need the `public.toml` of a running cothority where
you have the right to create a skipchain or add new blocks. If you want only
to test how it works, the simplest way to get up and started is:

```bash
cd cothority/conode
./run_conode.sh local 3
```

This will start three conodes locally and create a new `public.toml` that
you can use with scmgr. In the following examples, we suppose that the
`public.toml` file is in the current directory and that you installed `scmgr`
using

```bash
go get github.com/dedis/cothority/scmgr
```

## Securing node by creating a link

Per default, everybody is allowed to use your conode to create new skipchains.
This is good for testing setups where your conode is in your private network
and cannot be accessed from the outside. However, if you put your conode on the
internet (and you should), you need to create a link with it first, which will
then remove the possibility for 3rd parties to create a new skipchain on your
conode. In a later step we'll let them access your conode for a restricted set
of tasks.

There are two ways to link with your conode: either you have access to the
`private.toml` file of your conode, or you can read its log-files and recover
a PIN.

### Link using private.toml

The `private.toml`-file is in 

## Creating a new skipchain

To create a new
