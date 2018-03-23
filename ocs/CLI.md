Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
[Onchain Secrets](README.md) ::
Onchain Secrets CLI

# Onchain Secrets CLI

This app interacts with the onchain-secrets service and allows for storing encrypted
files on the skipchain while only giving the key to registered readers.

The app needs a running cothority with the ocs service enabled to function.
Then you can:

- set up a new OCS skipchain
- evolve the roles: admin/reader
- join an OCS skipchain
- write a new file to the blockchain, where it is stored encrypted
- read a file from the blockchain

## Setting up

This set up has been tested on MacOSX, but it should also work on Linux. You
will create a directory for the experiment, download the code, create a
cothority and finally use the onchain-secrets skipchain.

If you haven't set up go 1.8 (or 1.9), please go to https://golang.org/doc/install and
follow the instructions. Then you can do:

```bash
cd ~
mkdir -p ocs
cd ocs
```

For all further directions in this README we suppose you're in your `~/ocs`
directory and have followed the steps one-by-one.

### Creating a local cothority

First you need to create a cothority, which is a set of nodes listening to requests
for services. Different services are available, but for now we only care about
the ocs-service.

```bash
go get -u -v github.com/dedis/onchain-secrets/conode
$(go env GOPATH)/src/github.com/dedis/onchain-secrets/conode/run_conode.sh local 3
```

This starts 3 conodes locally on your computer and writes a `public.toml`-file
containing the information necessary to connect to these nodes. The conodes
run in the background. If you want to stop them, use:

```bash
pkill -f conode
```

Starting them again is done using the `run_conode.sh` command from above. If
you want to verify if the conodes are correctly running, use the following
command:

```bash
conode check public.toml
```

# Basic operations

This gives an introduction to use the OCS-skipchains in an easy way. This
is not safe for a production system, as the skipchain has no access control
for writing, so anybody knowing the id of the skipchain can fill it up!

## Setting up an ocs-skipchain

Once the conodes are running, you can setup your first ocs-skipchain:

```bash
go get github.com/dedis/onchain-secrets/ocs
ocs manage create public.toml
```

## Writing a file to the skipchain

Before we write a document to the skipchain, we need a private/public
keypair for the reader that will be allowed to read it from the skipchain.
We can also add more than one reader, but let's start with one:

```bash
READER=$( ocs keypair )
READER_PRIV=$( echo $READER | cut -f 1 -d : )
READER_PUB=$( echo $READER | cut -f 2 -d : )
```

Lets get today's news and write the html-file to the skipchain, allowing
the holder of the private key we created to read it:

```bash
wget -Oindex.html https://news.ycombinator.com
FILE_ID=$( ocs write index.html $READER_PUB | grep Stored | cut -f 2 )
```

Now `index.html` is stored on the OCS-skipchain, encrypted with a symmetric key that
is stored on the skipchain. It also outputs the ID of the file that has been
stored on the skipchain and stores it in $FILE_ID.

## Reading the file from the skipchain

Reading a file from the skipchain is a two-step process, but `ocs`
does both steps with one call:

1. send a read-request to the skipchain
2. ask the cothority to re-encrypt the symmetric key

For the read-request we can give a file to output the decrypted file to.
If we don't give this `-o`-option, the file will be outputted to the
standard output. We also have to give the private key of the reader.
It will not be sent, but used for creating a signature to authenticate
to the cothority.

```bash
ocs read -o index_copy.html $FILE_ID $READER_PRIV
```

A verification using `cmp` shows it's the same file:

```bash
cmp index.html index_copy.html && echo The files are the same
```

## Joining an existing skipchain

Instead of running everything from a single account, you can also join an
existing skipchain. For this we add the `-c second` argument to `ocs`, so
that all configuration will be stored in the `second/`-directory (the default
directory is `~/.config/ocs`).

First we need the id of the skipchain:

```bash
OCS_ID=$( ocs manage list | cut -f 2 )
```

And then we can join the previous OCS-skipchain:

```bash
ocs -c second manage join public.toml $OCS_ID
```

Now we can request and fetch the file using this chain:

```bash
ocs -c second read -o index_copy2.html $FILE_ID $READER_PRIV
```

And again we can verify all files are the same:

```bash
cmp index.html index_copy2.html && echo Files are the same
```
