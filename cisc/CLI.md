Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
[CISC](README.md) ::
CLI

# CISC Command Line Interface

Here is a simple example how cisc can be used with two conodes. You can start
them on your local computer, and then run cisc against these two conodes. Of
course you can also start the conodes on your own server or laptop.

## Introduction

To better understand how `cisc` work, please look at the main README the parts
_Terms_ and _Block Content_. Furthermore, it can help to understand that there
are  three types of public/private key pairs involved in the CISC setup:

- conode-keys - the public keys of the conodes are stored in the public.toml
and are used in the conode-to-conode interaction to verify the authenticity of
the remote conode. The same keys are also used to collectively sign the new
blocks once they are accepted by the cothority.
- admin-keys - when the `cisc` command connects to a conode, he needs to
authenticate himself to proof that he has admin-rights, for example to set
up a new skipchain. The public key is stored on the remote conode and will
be used to verify if the request to create a new skipchain is valid or not.
- device-keys - every new device that joins a skipchain creates its own
public/private key pair that is used to sign new proposed data. Only when
a threshold of device-keys signs a proposed data will it be included as a
new block to the skipchain. The new block includes the individual signatures from the
device-keys, as well as a collective signature created by all the conodes
to indicate the new block is valid.

## Installing

To install the conode and start experimenting, we'll use a directory `~/conodes`
where everything will be stored:

```bash
go get github.com/dedis/cothority/cisc
cd $(go env GOPATH)/src/github.com/dedis/cothority/conode
./run_conode.sh .
```

### Starting Conodes

For starting two conodes, type the following:

```bash
cd $(go env GOPATH)/src/github.com/dedis/cothority/conode
./run_conode.sh local 2 2
```

### Stopping Conodes

If you want to stop the conodes, simply type:

```bash
pkill conode
```

## Using the conodes to store key/value pairs

### Connecting to one conode

Now we need to connect to one of the conodes. We'll use the PIN authentication:

```bash
cisc link pin localhost:7002
```

Now the server should print the PIN to the log. Because you started the conodes
in the background, the PIN will show up directly on your console. Now you can
link to it:

```bash
cisc link pin localhost:7002 123456
```

of course you'll have another pin than the one shown here. This command will
create a new private/public key pair to communicate with the remote conode. The
public key is stored in the remote conode, and administrative functions needing
authentication will be signed using the corresponding private key.

### Creating an identity

Now we're ready to create a new identity:

```bash
cisc skipchain create $(go env GOPATH)/src/github.com/dedis/cothority/conode/public.toml
```

or, if you're still in the conode-directory:

```bash
cisc skipchain create public.toml
```

The `public.toml` file gives the definition of the conodes that should be used
for this new skipchain. It holds the IP-address and the public key for each of
the conodes.
This will print an ID of your new skipchain, which is a 64-digit hex number.
We'll need it later to join the skipchain from another device.
When creating a new skipchain, a new block containing the name of the device and
its public key (generated automatically) is created and stored in the skipchain
service.

### Storing a key/value pair

The most simple is to store any key/value pair on the skipchain:

```bash
cisc keyvalue add name linus
```

This will add a key called _name_ with the value _linus_ to the skipchain and
vote for it. As there is only one device attached to the skipchain, the new data
is immediately active.

## Adding a second device

Normally a second device would join from another computer. For easier simulation,
we can give the configuration-directory to the `cisc` command and simulate a
second computer. Let's try to join the create skipchain from above:

```bash
cisc -config device2 skipchain join public.toml 1234...cdef device2
```

The `-config device2` indicates that the `cisc` command should now use the `device2`
directory for storage of all the configuration. This means that we can use
`cisc` to represent the 1st device, and `cisc -config device2` for the second device.
Again, the `public.toml` indicates the conodes to use to connect to the skipchain.
The number `1234...cdef` is the 64-digit hex number from the `cisc skipchain create`
command from above. The last argument, `device2`, sets the name of the new device
to be added to the skipchain. The command will automatically create a new
private/public keypair and add the public key to the proposed new configuration.

Before we can do anything with the skipchain from this second device, we need to
approve it from the first one:

```bash
cisc data vote
```

`cisc` will present the change in the configuration (new device added) and ask
you whether you want to accept it. If you press <enter>, the proposed data will
be signed with your key and sent to the skipchain. You can verify on the second
device that it has been added correctly:

```bash
cisc -config device2 data update
```

This should show that you're now also in the list of allowed devices.

### Adding a new key/value pair

From either the 1st or the 2nd device you can now ask for adding or changing
key/value pairs:

```bash
cisc keyvalue add phone 101
```

The 1st device proposed a new key _phone_ with the value _101_ and votes on it.
Before a new block on the skipchain is appended, it needs to be accepted by
the 2nd device:

```bash
cisc -config device2 data vote
```

Again it will show the new configuration and ask you to accept it. If you do,
the 1st device can see the changes:

```bash
cisc data update
```

And the new key/value pair should appear in the data-part.
