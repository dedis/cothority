# CoNode

This is the first implementation of a cothority node (CoNode) for public usage.
When built, it functions as a node in a static tree. This first implementation
serves as a public timestamper-server which takes a hash and returns the signed
hash together with the inclusion-proof.

A simple program is given that can get the hash of a file signed and check
the validity of a signature-file.

## Limitations / Disclaimer

Some known limitations that we would like to address as soon as possible:

* There's no exception-handling if a node is down.
* Each time you add nodes to your tree, the collective public signature changes.
* **BACKDOOR-POSSIBILITY:** the script `start-conode` downloads the latest
version of the CoNode-binaries, together with the definition of the tree. This
means we have to trust GitHub for not putting up something fancy, and you have
to trust us, we don't do it neither.

## Participate in the EPFL-CoNode

The most easy way to participate in the EPFL-CoNode is to use the pre-compiled
binary (for Linux64, Linux32, and MacOSX) available at:

https://github.com/dedis/cothority/releases/latest

These are the steps to be part in the EPFL-CoNode-project:

1. Download the binary distribution
2. Create the keypair and validate the installation
3. Start your CoNode
4. Stamp your documents
5. Updating the binaries

If you are more security-conscious, you can also refer to the *compile your own
version* section below.

### Download the Binary Distribution

Make a directory and cd into it:

```
mkdir conode
cd conode
```

Go to the page

https://github.com/dedis/cothority/releases/latest

and download the latest .tar.gz and untar it (replace with latest version)

```
wget http:///DeDiS/cothority/releases/download/0.5/conode-0.5.5.tar.gz
tar xf conode-0.5.5.tar.gz
```

### Create the Keypair and Validate the Installation

Now you're ready to create a new private/public key pair and start to validate
the installation. Best thing to do is to open a `screen` for background running:

```
screen -S conode
./start-conode setup <your IP>
```

This command will create a new key-pair and print the public key on the command
line. Please send that key to dev.dedis@epfl.ch and wait further instructions.
The command will wait for us to verify your installation, so please keep it
running. If you want to quit the `screen`, you can do so by typing
*<ctrl-a> + d*. To go back to your screen session, run

```
screen -r conode
```

If everything goes well and your CoNode is active for at least 24h, it will
automatically exit, get the tree-information and start to run in
CoNode-mode.

### Start your CoNode

Once the installation has been verified, it will change automatically into
running mode. If at a later time you stop it and want to restart it, use
the following command:

```
./start-conode run
```

### Stamp Some Documents

If everything is running correctly, you can start stamping documents:

```
./stamp sign file
```

Where *file* is the file you want to stamp. It will write the signature
and the inclusion-proof in `file.sig`.

To verify whether a document is correctly stamped and still valid, run

```
./stamp verify file
```

### Updating the Binaries

Every time the CoNode quits (for example the root-node quit), it searches on
GitHub to see if there is a new version of the script available. If it finds
a new one, it downloads it, untars it and re-launches the script.

This of course is a high security-risk. We do our best not to use your server
for anything else than running the CoNode. But there is no 100% guarantee!

## Participate in the EPFL-CoNode (Compile Your own Version)

These are the steps to be able to participate in the EPFL-CoNode-project:

0. Compile CoNode
1. Create a private/public-key pair
2. Send the public-key
3. Validate the installation
4. Start your CoNode
5. Stamp your documents

### Compile CoNode

We suppose you have a running Go implementation. To compile CoNode, you need
to have the repositories `dedis/crypto-library` and `dedis/cothority` in branch
`development`. Execute the following steps to get all the dependencies and
build CoNode:

```
go get https://github.com/dedis/cothority
cd $GOPATH/src/github.com/dedis/cothority
git checkout development
cd app/conode
go build
```

### Create Private/Public-key Pair

Once CoNode is compiled, you can call it with

```
conode keygen address [-key keyname]
```

where `address` is the public IP of your server. This will create two files in
your directory:

* `key.pub`
* `key.priv`

The `key.priv` is the secret for your CoNode and is not to be shared under any
circumstances!

If you add `-key keyname`, the files will be named `keyname.pub` and
`keyname.priv`, respectively.

### Send us the Configurations of your CoNode

For CoNode to work, you need access to a server that is running permanently
and has a public IP-address. CoNode uses ports 2000 (for internal
communication) and 2001 (for stamp-requests) for communication. So
if you have set up a firewall, these ports must be opened.

Then send the IP-address of your server together with the public key to
dev.dedis@groups.epfl.ch with the subject "New CoNode".

### Validate the Installation

Before we will add you to our tree of CoNodes, we want to make sure that your
server is up and running for at least 24 hours. For this, run CoNode with the
following command:

```
screen -S conode -dm conode validate
```

This will run CoNode on the address as given in the step *Create
Private/Public-key Pair* above using the files `key.pub` and `key.priv`. If you
changed the key name, don't forget to add `-key keyname` to the above command.

Now CoNode should be working and waiting for us to check the correct setup of
it. If you want to return to your screen-session, you can always type

```
screen -r conode
```

and look at the output of CoNode to see whether there has been something
coming in. To quit `screen`, type *<ctrl-a> + d*.

### Start CoNode

If everything is setup correctly and you're accepted by our team as a
future CoNode, we will send you a `config.toml`-file that has to be copied
to the CoNode-directory. Once it is copied there, you can restart
CoNode:

```
screen -S conode -dm conode run
```

Be sure to stop the other CoNode first before running the above command,
otherwise `screen` will tell you that a session with the name `conode` already
exists. By default, it will try to read the config file `config.toml`.
However, if you have another name for the config file, please specify it with
the option `-config configFile`.
Again, if you changed the key-file name, don't forget to add
`-key keyname`

### View Stats

CoNode will write some usage statistics on the standard output which can be
viewed with

```
screen -r conode
```

To exit `screen`, type *<ctrl-a> + d*.

## Stamp Documents

There is a simple stamper-request-utility that can send the hash of
a document to your timestamper and wait for it to be returned once
the hash has been signed. This is in the `stamp`-subdirectory.
Before you can use it, you have to copy the `config.toml`-file
to that directory, too.

After compilation, you can be use it like this:

```
./stamp sign file
```

Where `file` is the file you want to stamp. It will write the signature
and the inclusion-proof in `file.sig`.

### Verify the Signature of a Document

The signature is linked to the actual CoNode-tree, to check whether it
has been signed by it, you can use

```
./stamp check file
```

If `file` is present, its hash-value is verified against the value stored
in `file.sig`, otherwise only the information in `file.sig` is verified.

## Set up Your own CoNodes

If you want to create your own tree of CoNodes, there are two additional
commands that will help you:

* `check` - to check whether a new node is available
* `build` - to create a `config.toml`-file

### Check Node Availability

When you receive a request from a new user to participate in your CoNode, it
is best practice to make sure that his server is available for some time.
Rename every `key.pub`-file you receive to something useful for you and put it
in the CoNode-directory.

We propose to watch the availability of his node over a period of at least 24h.
To check, whether his server is up and running, type the following

```
./conode check keyname.pub
```

where `keyname.pub` is the public key file of the user. CoNode will verify that
the server is available at the given address and that its private key
corresponds to the given public key.

### Build - Create a Config File

Once you checked the server to a corresponding `key.pub`-file is available,
you can concatenate them to build a host-list, then pass it to the
CoNode-binary:

```
cat key.pub > hostlist
./conode build hostlist
```

Now you can pass the generated `config.toml`-file to all your users who
have to restart their CoNode in `run`-mode.

### Restart the CoNode

We don't have yet an automatic way of restarting all servers, so we propose
you ask all your users to restart their servers with the new `config.toml`-
file at a given time.

## Some Technical Info

You can find some more technical infos for example at:

https://petsymposium.org/2015/papers/syta-cc-hotpets2015.pdf

### config.toml

The file `config.toml` contains:

* The suite used (for the moment we stick to the Edwards-25519-curve)
* A list of all hosts
* The aggregate public key of the CoNode
* The tree of all hosts together with the public-key of each host

The aggregate public key changes from one CoNode-installation to another
(even adding / removing a node will change it).

### file.sig

If you created a signature for a file, here is a short explanation of the
fields you will find in that file:

* name - the name of the file
* hash - a SHA256-based hash
* proof - the inclusion-proof for your document
* signature - the collective signature

If you want to verify a signature, you will also need the `config.toml` which
contains the aggregate public key.

## Contact us

If you are running your own CoNodes, we would be very happy to hear from you. Do
not hesitate to contact us at

dev.dedis@epfl.ch

and tell us who you are and what you're doing with our CoNodes.
