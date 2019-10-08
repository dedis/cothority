Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/doc/BuildingBlocks.md) ::
[ByzCoin](https://github.com/dedis/cothority/blob/master/byzcoin/README.md) ::
bcadmin

# bcadmin - the CLI to configure ByzCoin ledgers

## Example usage

Let's suppose an _admin_ wants to start a new byzcoin blockchain and give
access to a user _foo_, so that _foo_ can onboard new users, too, but cannot
change the byzcoin-configuration.

There is a set of nodes stored in `roster.toml` that are online and ready 
to accept new byzcoins.

Commands on _admin_'s machine are prepended with `admin $ `, while commands
on _foo_'s machine are prepended with `foo $ `

### Create a new ByzCoin

First the admin will create a new byzcoin and get a private key so he can
interact with the blockchain:

```bash
admin $ bcadmin -c . create -roster roster.toml 
```

This stores the byzcoin-configuration holding the roster, the byzcoin-id,
as well as the darc and the identity of the _admin_ in a file called
`bc-xxxx.cfg`, where `xxxx` is the id of the new byzcoin-instance. 
The private key of the _admin_ is stored in `key-ed25519:pub_admin.cfg`, 
where `pub_admin` is the public key of the _admin_. The `-c $dir` flag means
the key will be created in the directory `$dir`. If the `-c` flag is not used,
the key will be created in the default configuration directory. 

Currently it is not possible for the admin to specify the key, the `create`
sub-command always generates a new one.

### Sign up a new user

To sign up the new user _foo_, he needs to create a new keypair:

```bash
foo $ bcadmin -c . key
```

This will create a `key-ed25519:pub_foo.cfg` file, where `pub_foo` is in fact the public key
of the new user. Now _foo_ needs to send `pub_foo` to the admin, so that he can
create a new darc for the user _foo_:

```bash
admin $ bcadmin -c . darc add --bc bc-xxxx.cfg --unrestricted --sign ed25519:pub_admin \
                --owner ed25519:pub_foo --desc "Darc for Foo"  
```

This command will output the darc-id `darc:darc_foo` that has been created. The _admin_ can
now transfer this darc-id to the user _foo_.

### Using the new darc

Now for _foo_ to use this new darc, he will first have to create a configuration-file
`bc-xxxx.cfg` with the new darc inside:

```bash
foo $ bcadmin -c . link roster.toml xxxx --pub ed25519:pub_foo --darc darc:darc_foo
``` 

This will create a file `bc-xxxx.cfg` in _foo_'s directory that points to _foo_'s
darc. If _foo_ wants to onboard other users, he needs to evolve his darc to allow
for spawning new darcs:

```bash
foo $ bcadmin -c . darc rule --bc bc-xxxx.cfg --darc darc:darc_foo --sign ed25519:pub_foo \
                --rule spawn:darc --identity ed25519:pub_foo
```

If everything was OK, _foo_ has now the possibility to sign up _bar_.

## Command reference

### Create a new ByzCoin, saving the config

```
$ bcadmin create -roster roster.toml
```

The `roster.toml` file is a list of servers what form the cothority that will
maintain the ledger. After running `run_conode.sh local 3` for example, the file
`public.toml` will have the 3 conodes in it. For a larger production deployment,
you will construct the `roster.toml` file by collecting the `public.toml` files
from each of the servers.

The ByzCoin config info (the skipchain ID and the roster for the cothority)
are stored in the local config directory (~/.config/bcadmin or ~/Library/Application
Support/bcadmin) and the filename is printed on stdout. The ByzCoin config file
will be used by other tools to know where to send their transactions. It has no
seret information in it.

The secret key is saved in a file named after the public key. It must not be
shared!

To see the config you just made, use `bcadmin show -bc $file`.

### Granting access to contracts

The user who wants to use ByzCoin generates a private key and shares the
public key with you, the ByzCoin admin. You grant access to a given contract
for instructions signed by the given secret key like this:

```
$ bcadmin darc rule -bc $file -rule spawn:theContractName -identity ed25519:dd6419b01b49e3ffd18696c93884dc244b4688d95f55d6c2a4639f2b0ce40710
```

Different contracts will require different permissions. Check
their docs. Usually they will need at least "spawn:$contractName" and
"invoke:$contractName".

Using the ByzCoin config file you give them and their private key to sign
transactions, they will now be able to use their application to send
transactions.

### Environment variables

You can set the environment variable BC to the config file for the ByzCoin
you are currently working with. (Client apps should follow this same standard.)

### Generating a new keypair

```
$ bcadmin key
```

Generates a new keypair and prints the result in the console

Optional flags:

-save file.txt            Outputs the key in file.txt instead of stdout

### Managing DARCS

```
$ bcadmin darc add -bc $file
```

Adds a new darc with a random keypair for both signing and evolving it.

Optional flags:

 * -out_id file.txt          Outputs the ID of the DARC in file.txt
 * -out_key file.txt         Outputs the key of the DARC in file.txt
 * -out file.txt             Outputs the full description of the DARC in file.txt
 * -owner key:%x             Creates the DARC with the mentioned key as owner (sign & evolve)
 * -darc darc:%x             Creates the DARC using the mentioned DARC for creation (uses Genesis DARC by default)
 * -sign key:%x              Uses this key to sign the transaction (AdminIdentity by default)
 * -desc description         The description for the new DARC (default: random)
 * -unrestricted             Add the invoke:evolve_unrestricted rule

```
$ bcadmin darc show -bc $file
```

Shows a DARC either in stdout or in a given file

Optional flags:

 * -out file.txt             Outputs the description of the DARC in file.txt instead of stdout
 * -darc darc:%x             Shows the DARC with provided ID, Genesis DARC by default

```
$ bcadmin darc rule -bc $file -rule $action
```

Manages rules for a DARC

Optional flags:
 * -darc darc:%x             Modifies the rules of this DARC (uses Genesis DARC by default)
 * -sign key:%x              Uses this key to sign the transaction (AdminIdentity by default)
 * -delete                   Deletes the specified rule if it exists
 * -identity:%x              The expression that will determine the necessary signatures to perform the action (mandatory if -delete is not used)
 * -replace                  Overwrites the expression for the necessary signatures to perform the action (if not provided and action already exists in Rules the action will fail)

 ```
 $ bcadmin darc
 ```

 is equivalent to `show`.

 ```
 $ bcadmin qr
 ```

Displays a QR Code containing the ByzCoin configuration, compatible to be scanned by the PopCoins apps

Optional flags:
 * -admin   The QR Code will also contain the admin keypair to allow the user who scans it to manage the ByzCoin

## Debug usage

To debug issues with ByzCoin, `bcadmin` supports commands to poke the chain
on the block-level to better understand eventual errors.

### View Block

It is possible to display a block from one or all nodes of the chain with the
 following command:
 
```bash
$ bcadmin debug block --bc bc-xxx.cfg --index 0 --txDetails
```

This command will show the genesis-block of the chain defined in `bc-xxx.cfg`
 of all nodes, and also show the transactions contained in that block.

## DataBase Methods

Bcadmin can also work on the database - either a separate, or a database from
 a conode. The following commands are available:
 
- `db merge` copies the skipblocks from one database to another
- `db catchup` fetches new blocks from the network
- `db replay` applies the blocks from the database to the global state
- `db status` returns simple status' about the internal database
- `db check` goes through the whole chain and reports on bad blocks

Before a release of a new version, the following commands should be run 
and return success:

```bash
bcadmin db catchup cached.db _bcID_ _url_
bcadmin db replay cached.db _bcID_ --continue
```

The `_bcID_` has to be replaced by the hexadecimal representation of the 
chain to be tested (for the DEDIS chain: 
`9cc36071ccb902a1de7e0d21a2c176d73894b1cf88ae4cc2ba4c95cd76f474f3`) and the 
`_url_` can be any node in the network who has the needed blocks available, 
e.g., `https://conode.dedis.ch`.

### Creating a full node out of a caught-up node

If a node is stuck, sometimes the only way to continue is to delete its 
database and restart it. This works fine, but the node then only has the 
minimal set of needed blocks and can not participate in catching-up for 
replays.

Suppose a node has been restarted with an empty database, and has caught up, 
then you can use the following to store all blocks in the node again:

```bash
# First you need to stop the node, else the db cannot be modified
bcadmin db catchup path/to/conode.db _bcID_ _url_
# Then start the node again
```

If no other node in the system has all nodes stored, then you can `merge` a 
db with all nodes:
```bash
# First stop the node
bcadmin db merge --overwrite path/to/conode.db _bcID_ cached.db
```

The `--overwrite` is necessary to store all blocks from the `cached.db` file 
to the existing database.

A `cached.db` is available at https://conode.c4dt.org/files/cached.db