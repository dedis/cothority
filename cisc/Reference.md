Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Applications](../doc/Applications.md) ::
[CISC](README.md) ::
CLI Command Reference

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
  * Leave - remove this device from an identity
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
  * Sync - interactively syncs your `~/.ssh/config` file to or from the skipchain
  * Rotate - rotate all your keys

## cisc keyvalue
The kv-data-type simply holds a map of key/value pairs that are shared by all
devices of the identity. This can be for example the login/password, where the
password would be encrypted with a master-password of course.

cisc kv has the following subcommands:
  * List - returns a list of all keys pairs
  * Value - returns the value of a given key
  * Add - adds a key/value pair by proposing the new data to the identity
  * Del - removes a key/value pair by proposing the new data to the identity
  * File - reads a list of key/value pairs from a CSV file

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
