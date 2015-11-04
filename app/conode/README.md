# Conode

This is the first implementation of a Cothority Node for public usage. When
built it functions as a node in a static tree. This first implementations
serves as a public timestamper-server which takes a hash and returns the
signed hash together with the inclusion-proof.

A simple program is given that can get the hash of a file signed and check
the validity of a signature-file.

## Limitations / Disclaimer

Some known limitations that we would like to address as soon as possible:

* No exception-handling if a node is down
* Each time you add nodes to your tree, the collective public signature changes
* BACKDOOR-POSSIBILITY: the script ```start-conode``` downloads the latest
version of the conode-binaries, together with the definition of the tree. This
means we have to trust github for not putting up something fancy, and you have
to trust me I don't do it neither.

## Participate in the EPFL-conode

The most easy way to participate in the EPFL-conode is to use the precompiled
binary (for linux/64 linux/32 and MacOSX) available at:

https://github.com/dedis/cothority/releases/latest

If you are more security-conscious, you can also go to 'compile your own version'
section.

These are the steps to be part in the EPFL-conode-project:

1. Download the binary distribution
2. Create the keypair and validate the installation
3. Start your conode
4. Stamp your documents
5. Updating the binaries

### Download the binary distribution

Make a directory and cd into it:

```mkdir conode
cd conode```

Go to the page

https://github.com/dedis/cothority/releases/latest

and download the latest .tar.gz and untar it (replace with latest version)

```wget http:///DeDiS/cothority/releases/download/0.5/conode-0.5.5.tar.gz
tar xf conode-0.5.5.tar.gz```

### Create the keypair and validate the installation

Now you're ready to create a new private/public key pair and start to validate
the installation. Best thing to do is to open a ```screen``` for background
running:

```screen -S conode
./start-conode setup <your IP>```

This command will create a new key-pair and print the public key on the command
line. Please send that key to dev.dedis@epfl.ch and wait further instructions.
The command will wait for us to verify your installation, so please keep it
running. If you want to quit the ```screen```, you can do so by typing
```<ctrl>a + d```. To go back to your screen session, run

```screen -r conode```

If everything goes well and your conode is active for at least 24h, it will 
automatically exit, get the tree-information and start to run in
conode-mode.

### Start your conode

Once the installation has been verified, it will change automatically into
running mode. If at a later time you stop it and want to restart it, use
the following command:

```./start-conode run```

### Stamp some documents

If everything is running correctly, you can start stamping documents:

```./stamp sign file```

Where *file* is the file you want to stamp. It will write the signature
and the inclusion-proof in ```file.sig```.

To verify whether a document is correctly stamped and still valid, run

```./stamp verify file```

### Updating the binaries

Every time the conode quits (for example the root-node quit), it searches on
github to see if there is a new version of the script available. If it finds
a new one, it downloads it, untars it and re-launches the script.

This of course is a high security-risk. We do our best not to use your server
for anything else than running the conode. But there is no 100% guarantee!

## Participate in the EPFL-conode - compile your own version

These are the steps to be part in the EPFL-conode-project:

0. Compile Conode
1. Create a private/public-key pair
2. Send the public-key
3. Validate the installation
4. Start your conode
5. Stamp your documents

### Compile Conode

For Conode to compile, you need to have the dedis/crypto-library in branch
```shamir``` and the conode in branch ```development```. We suppose you have
a running 

```go get https://github.com/dedis/cothority
cd $GOPATH/src/github.com/dedis/crypto
git checkout shamir
cd ../cothority
git checkout development
cd app/conode
go build
```

### Create private/public-key pair

Once conode is compiled, you can call it with

```conode keygen address [-key keyname]```

Where ```address``` is the public IP of your server. This will create two 
files in your directory:

* key.pub
* key.priv

The key.priv is the secret for your conode, this is not to be shared under 
any circumstances!

If you add ```-key keyname```, the files will be named keyname.pub and
keyname.priv respectively.

### Send the public-key

For Conode to work, you need access to a server that is running permanently
and has a public IP-address. Conode uses the port 2000 (for internal 
communication) and 2001 (for stamp-requests) for communication, so
if you have set up a firewall, these ports must be opened.

Then send the IP-address of your server together with the public key to
dev.dedis@groups.epfl.ch with a subject of "New conode".

### Validate the installation

Before we will add you to our tree of conodes, we want to make sure that your
server is up and running for at least 24 hours. For this, run conode with the
following command:

```screen -S conode -dm conode validate```

This will run conode on the address given in the step 'Create private/public-key
pair' with the file key.pub/key.priv. If you changed the keyname, don't forget to
add ```-key keyname``` to the above command.

Now conode should be working and waiting for us to check the correct
setup of it. If you want to return to your screen-session, you can type
at any moment

```screen -r conode```

and look at the output of conode to see whether there has been something
coming in. To quit 'screen', type *<ctrl-a> + d*

### Start your conode

If everything is correctly setup and you're accepted by our team as a
future conode, we will send you a config.toml-file that has to be copied
to the conode-directory. Once it is copied there, you can restart
conode:

```screen -S conode -dm conode run```

Be sure to stop the other conode first before running that command.
Else 'screen' will tell you that a session with the name 'conode'
already exists.
By default, it will try to read the config file "config.toml".
However, if you have another name for the config file, please specify it with
the option ```-config configFile```.
Again, if you changed the key-file name, don't forget to add
```-key keyname```

### Enjoy

Conode will write some usage statistics on the standard output which can be
viewed with

```screen -r conode```

To exit screen, type *<ctrl-a> + d*

## Stamp your documents

There is a simple stamper-request-utility that can send the hash of
a document to your timestamper and wait for it to be returned once
the hash has been signed. This is in the ```stamp```-subdirectory.
Before you can use it, you have to copy the ```config.toml```-file
to that directory, too.

After compilation, you can be use it like this:

```./stamp sign file```

Where *file* is the file you want to stamp. It will write the signature
and the inclusion-proof in ```file.sig```.

### Verify the signature of a document

The signature is linked to the actual conode-tree, to check whether it
has been signed by it, you can use

```./stamp check file```

If ```file``` is present, it's hash-value is verified against the value stored
in ```file.sig```, else only the information in ```file.sig``` is verified.

## Set up your own conodes

If you want to create your own tree of conodes, there are two additional
commands that will help you:

- ```check``` - to check whether a new node is available
- ```build``` - to create a ```config.toml```-file

### Check whether a node is available

When you receive a request from a new user to participate in your conode, it
is best practice to make sure that his server is available for some time.
Rename every key.pub-file you receive to something useful for you and put it
in the conode-directory.

We propose to watch the availability of his node over a period of at least 24h.
To check, whether his server is up and running, type the following:

```./conode check keyname.pub```

```keyname.pub``` is the public keyfile of the user. Conode will verify that the
server is available at the given address and whether his private key corresponds
to the given public key.

### Build - create config-file

Once you checked the server to a corresponding key.pub-file is available,
you can concatenate them to build a host-list, then pass it to the 
```conode```-binary:

```cat key*pub > hostlist```
```./conode build hostlist```

Now you can pass the generated ```config.toml```-file to all your users who
have to restart their conode in ```run```-mode.

### Restart the conode

We don't have yet an automatic way of restarting all servers, so we propose
you ask all your users to restart their servers with the new config.toml-
file at a given time.

## Some technical info

You can find some more technical infos for example at:

https://petsymposium.org/2015/papers/syta-cc-hotpets2015.pdf

### config.toml

This config-file contains:

* The suite used (for the moment we stick to the Edwards-25519-curve)
* A list of all hosts
* The aggregate public key of the conode
* The tree of all hosts together with the public-key of each host

The aggregate public key changes from one conode-installation to another
(even adding / removing a node will change it).

### file.sig

If you created a signature for a file, here is a short explanation of the
fields you will find in that file:

* name - the name of the file
* hash - sha-256-based hash
* proof - the inclusion-proof for your document
* signature - the collective signature

If you want to verify a signature, you will also need the config.toml
which contains the aggregate public key.

## Contact us

If you set up your own conode, we would be very happy to hear from you.
Best is, you contact

dev.dedis@epfl.ch

And tell us who you are and what you're doing with our conodes.
