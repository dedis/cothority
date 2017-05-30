# WLogR - WriteLogRead

This app interacts with the logread-service and allows for storing encrypted
files on the skipchain while only giving the key to signed up readers.

The app needs a running cothority with the logread-service enabled to function.
Then you can:

- set up a new pair of ACL/WLR-skipchains
- evolve the roles: admin/writer/reader
- join a logread-skipchain as one of admin/writer/reader
- write a new file to the blockchain, where it is stored encrypted
- read a file from the blockchain

## Setting up

This set up has been tested on MacOSX, but it should also work on Linux. You
will create a directory for the experiment, download the code, create a
cothority and finally use the logread-skipchain.

If you haven't set up go 1.8, please go to https://golang.org/doc/install and
follow the instructions. Then you can do:

```bash
cd ~
mkdir -p logread
cd logread
```

For all further directions in this README we suppose you're in your `~/logread`
directory and have followed the steps one-by-one.

## Creating a local cothority

First you need to create a cothority, which is a set of nodes listening to requests
for services. Different services are available, but for now we only care about
the logread-service.

```bash
go get github.com/dedis/cothority/conode
$GOPATH/src/github.com/dedis/cothority/conode/run_conode.sh local 3
```

This starts 3 conodes locally on your computer and writes a `public.toml`-file
containing the information necessary to connect to these nodes. The conodes
run in the background. If you want to stop them, use:

```bash
pkill -f -9 conode
```

Starting them again is done using the `run_conode.sh` command from above. If
you want to verify if the conodes are correctly running, use the following
command:

```bash
conode check public.toml
```

## Setting up a logread-skipchain

Once the conodes are running, you can setup your first logread-skipchain:

```bash
go get github.com/dedis/cothority/logread/wlogr
wlogr manage create public.toml manager
```

This also prints the ID of the WLR-skipchain that you need for the next steps.
You can store it like this:

```bash
WLR_ID=$( wlogr manage list | cut -f 2 )
```

## Adding a writer and a reader

Now we need somebody to write and somebody to read the file:

```bash
wlogr manage role create writer:alice
wlogr manage role create reader:bob
```

This prints out the private keys of the users that you need for accessing the
skipchain from another account. For easier handling, you can store them:

```bash
ALICE=$( wlogr manage role list | grep alice | cut -f 2 )
BOB=$( wlogr manage role list | grep bob | cut -f 2 )
```

## Writing a file to the skipchain

Let's get today's news and write the html-file to the skipchain:

```bash
wget https://news.ycombinator.com
FILE_ID=$( wlogr write alice index.html | grep Stored | cut -f 2 )
```

Now `index.html` is stored on the WLR-skipchain, encrypted with a symmetric key that
is stored on the skipchain. It also outputs the ID of the file that has been
stored on the skipchain and stores it in $FILE_ID.

## Reading the file from the skipchain

To get the file back from the skipchain, a reader first needs to make a read-request,
before he can fetch the file from the skipchain and get the appropriate
decryption-key.

```bash
READ_ID=$( wlogr read request bob $FILE_ID | grep Request-id | cut -f 2 )
```

Now there is a new block on the WLR-skipchain with a log that says `bob` requested
read access to file `$FILE_ID`. Bob can now get the file:

```bash
wlogr read fetch $READ_ID index_copy.html
```

A verification using sha256 shows it's the same file:

```bash
shasum -a 256 index.html index_copy.html
```

## Joining an existing skipchain

Instead of running everything from a single account, you can also join an
existing skipchain. For this we add the `-c second` argument to `wlogr`, so
that all configuration will be stored in the `second/`-directory (the default
directory is `~/.config/wlogr`).

So let's join the previous WLR-skipchain with Bob's reading permissions:

```bash
wlogr -c second manage join public.toml $WLR_ID $BOB
```

Now Bob can request and fetch the file using this chain:

```bash
READ_ID2=$( wlogr -c second read request bob $FILE_ID )
wlogr -c second read fetch $READ_ID2 index_copy2.html
```

And again we can verify all files are the same:

```bash
shasum -a 256 index*html
```
