# Conode

This is the first implementation of a Cothority Node for public usage. When
built it functions as a node in a static tree. This first implementations
serves as a public timestamper-server which takes a hash and returns the
signed hash together with the inclusion-proof.

## Usage

These are the steps to be part in the conode-project:

1. Create a private/public-key pair for your node
2. Send the public-key together with your server-IP to dev.dedis@groups.epfl.ch
3. Run conode and wait for acknowledgement
4. Copy the received tree.json to your conode-directory and start conode
5. Enjoy

### Create private/public-key pair

Once conode is compiled, you can call it with

```conode --create-key```

This will create two files in your directory:

* publick.key
* private.key

The private.key is the secret for your conode, this is not to be shared under 
any circumstances!

If the files already exist in your directory, conode will quit and ask you to
delete them first.

### Sending the public-key

For Conode to work, you need access to a server that is running permanently
and has a public IP-address. Conode uses the port 2000 for communication, so
if you have set up a firewall, this port must be passing through.

Then send the IP-address of your server together with the public key to
dev.dedis@groups.epfl.ch with a subject of "New conode".
 
### Run conode
 
From the moment of sending your public-key to our mailing-list,
your conode has to be up and running, so that we can check everything
is correctly setup. To run your conode on Linux, we propose using 
'screen':

```screen -S conode -Hdm conode --check-setup```

Now conode should be working and waiting for us to check the correct
setup of it. If you want to return to your screen-session, you can type
at any moment

```screen -r conode```

and look at the output of conode to see whether there has been something
coming in. To quit 'screen', type *<ctrl-a> + d*

### Copy tree.json

If everything is correctly setup and you're accepted by our team as a
future conode, we will send you a tree.json-file that has to be copied
in the conode-directory. Once it is copied in there, you can restart
conode:

```screen -S conode -Hdm conode --run```

Be sure to stop the other conode first before running that command.
Else 'screen' will tell you that a session with the name 'conode'
already exists.

### Enjoy

Conode will write some usage statistics on the standard output and to
the ```conode.log```-file which you can watch by typing

```tail -f conode.log```

## Stamp your documents

We created a simple stamper-request-util that can send the hash of
a document to your timestamper and wait for it to be returned once
the hash has been signed. This is in the ```stamp```-subdirectory
and can be used like this:

```stamp file conode-ip```

Where *file* is the file you want to stamp and the *conode-ip* is the
IP-address of your conode or any other conode in the tree. It will
output your hash, the inclusion proof and the signature.