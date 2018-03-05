# CISC - Cisc Identity SkipChain

## Description
Cisc uses a personal blockchain handled by the cothority. It
 can store key/value pairs, and has a special module for managing
 ssh-public-keys.

Based upon skipchains, cisc serves a data-block with different entries that can
be handled by a number of devices who propose changes and cryptographically vote
to approve or deny those changes. Different data-types exist that will interpret
the data-block and offer a service.

Besides having devices that can vote on changes, simple followers can download
the data-block and get cryptographically signed updates to that data-block to be
sure of the authenticity of the new data-block.

- CISC - CISC Identity SkipChain
- Skipchain - blockchain structure developed by the EPFL/DEDIS lab
- Conode - a server program offering services like CISC and others
- Device - a computer or smartphone that has voting power on an identity-skipchain
- Data - all key/value pairs stored on the SkipChain
- Proposed Data - data that has been proposed but not yet voted with a threshold

# Example usage

Here is a simple example how cisc can be used with two conodes. You can start
them on your local computer, and then run cisc against these two conodes. Of
course you can also start the conodes on your server.

## Installing

To install the conode and start experimenting, we'll use a directory `~/conodes`
where everything will be stored:

```bash
go get github.com/dedis/cothority/conode
go get github.com/dedis/cothority/cisc
cd $(go env GOPATH)/src/github.com/dedis/cothority/conode
./run_conode.sh .
```

## Starting conodes and setting up the first device

For starting two conodes, type the following:

```bash
cd $(go env GOPATH)/src/github.com/dedis/cothority/conode
./run_conode.sh local 2 2
```

If you want to stop the conodes, simply type:

```bash
pkill conode
```

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

## Public/Private keys

There are three types of public/private key pairs involved in the CISC setup:
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

# Command reference

Cisc takes different commands and sub-commands with arguments. The main commands are:
  * Link - manages the authentication towards conodes
  * Skipchain - manages the identities this device is connected to
  * Data - handles the data of the identities this device is connected to
  * Keyvalue - direct key/value pair editing
  * Ssh - interfaces the ssh-data of the identities
  * Follow - for servers or other computers that want to follow a Skipchain
  * Web - add a web-page to the SkipChain

## cisc link

Commands to connect to conodes and save the authentication data there. To connect and store date, you need to use `cisc link` followed by:
  * Pin - connect to a conode by providing a PIN
  * Addfinal - adds a final statement from a pop-party to authorize creating new skipchains
  * Addpublic - adds a public key to authorize creating new skipchains
  * keypair - creates a private/public keypair for use with `cisc link addpublic`
  * list - shows a list of all links stored on this client

The difference between `cisc link pin` and `cisc link add(final|public)` is that the
first links with administrator rights (allowed to add other links), while the second
two link only with the rights to add new skipchains.

## cisc skipchain

Each device can be connected to multiple identities. As long as you only have one
identity linked, you don't need to give the id when working with it. Only if there
are multiple identities linked, you need to give the id of the identity you want
to work with. You can manage the connections with `cisc skipchain` followed by:
  * Create - asks the skipchain to create a new identity and returns its id #. It also connects to that identity.
  	Users have to authenticate to get possibility to create skipchain. Current implementation supports two ways of authentication:
	   * Pop-Token - in this case user will keep privacy - service won't know who creates the skipchain
	   * Public keys - no privacy, but no pop-party visit is required
  * Join - will ask the devices of the remote skipwchain to vote on the inclusion of this device in the skipchain
  * Del - remove this device from an identity
  * Roster - change the roster of an identity
  * List - show all stored skipchains
  * QRCode - prints a QRCode on the terminal so a remote device can contact

## cisc data
Each identity, connected or linked, has data attached to it that can be updated,
modified and voted upon. To modify the data, you need to use the appropriate
commands (`cisc ssh` and `cisc kv`).
Every time the data is changed, a _proposition_ is created, that has to
been voted upon by a majority of devices. To update and vote, you can use `cisc data`
followed by:
  * Clear - remove all proposed new data
  * Update - fetches the latest data from all identities from the skipchain as
  well as all proposed data (the ones which are not yet voted upon with a threshold)
  * List - updates and lists all data associated with all identities
  * Vote - sends a positive vote or a rejection for a specific update-proposition

## cisc ssh
The ssh-data-type allows for an easy handling of multiple ssh-identities over a
range of devices. It uses the ~/.ssh/config to get the list of ssh-identities
on this device. In addition to the usual configurations, each ssh-identity can
be preceded by a commented line
```
#cisc - ID#
```
Which indicates what identity shall be used to follow that ssh-identity.
This is useful for configuring groups of identities that share some hosts.

The different sub-commands for `cisc ssh` are:
  * Add - creates a new entry in the ~/.ssh/config file for a new host and proposes the new data
  * Del - removes an entry in the ~/.ssh/config file and proposes the new data
  * List - shows all connections for this device

## cisc kv
The kv-data-type simply holds a map of key/value pairs that are shared by all
devices of the identity. This can be for example the login/password, where the
password would be encrypted with a master-password of course.

cisc kv has the following subcommands:
  * List - returns a list of all keys pairs
  * Value - returns the value of a given key
  * Add - adds a key/value pair by proposing the new data to the identity
  * Del - removes a key/value pair by proposing the new data to the identity

## cisc follow
A server can set up cisc to follow a skipchain and update the
`authorized_keys.cisc`-file whenever a change in the list of ssh-keys occurs.
For convenience, cisc writes to `authorized_keys.cisc`, so that you can keep
your own keys, too. If you don't have a `authorized_keys`-file, cisc will
create a symlink that points to `authorized_keys.cisc`. In case you want
to use both files, we suggest that you add

```conf
AuthorizedKeysFile ~/.ssh/authorized_keys ~/.ssh/authorized_keys.cisc
```

to your `/etc/ssh/sshd_config`. Now sshd will read both files and allow
any key that is present in either of the files.

`cisc follow` has the following subcommands:
  * Add - takes `group.toml`, `Skipchain-ID` and `service-name` as an
  argument. It connects to one of the servers in `group.toml` and fetches
  the skipchain with `Skipchain-ID`. All ssh-keys referencing `service-name`
  will be written to `~/.ssh/authorized_keys.cisc`
  * Del - takes `Skipchain-ID` as an argument and removes the reference to
  that skipchain
  * List - prints a list of all connected skipchains and the keys stored
  in them
  * Update [-p interval] - looks for updates of one of the skipchains. In
   case it finds a change in the ssh-keys, it will update
   `~/.ssh/authorized_keys.cisc`

## cisc web
A skipchain can also hold a set of webpages that are stored and updated only when
a threshold of devices vote on the update. A client can follow the skipchain and
always have the updated webpage on his device. Viewers are for the moment available
for browsers (http://status.dedis.ch) and for mobile devices.

The only command for web is `cisc web` which takes a path to a html-file that will
be put on the skipchain.
