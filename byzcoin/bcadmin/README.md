Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/doc/BuildingBlocks.md) ::
[ByzCoin](https://github.com/dedis/cothority/blob/master/byzcoin/README.md) ::
bcadmin

# bcadmin - the CLI to configure ByzCoin ledgers

## Create a new ByzCoin, saving the config

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

## Granting access to contracts

The user who wants to use ByzCoin generates a private key and shares the
public key with you, the ByzCoin admin. You grant access to a given contract
for instructions signed by the given secret key like this:

```
$ bcadmin add -bc $file spawn:theContractName -identity ed25519:dd6419b01b49e3ffd18696c93884dc244b4688d95f55d6c2a4639f2b0ce40710
```

Different contracts will require different permissions. Check
their docs. Usually they will need at least "spawn:$contractName" and
"invoke:$contractName".

Using the ByzCoin config file you give them and their private key to sign
transactions, they will now be able to use their application to send
transactions.

## Environmnet variables

You can set the environment variable BC to the config file for the ByzCoin
you are currently working with. (Client apps should follow this same standard.)

## Generating a new keypair

```
$ bcadmin key
```

Generates a new keypair and prints the result in the console

Optional flags:

-save file.txt            Outputs the key in file.txt instead of stdout
## Managing DARCS

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

 is equivalent to show

 ```
 $ bcadmin qr
 ```

Generates a PNG QR Code containing the ByzCoin configuration, compatible to be scanned by the PopCoins apps

Optional flags:
 * -admin                   The QR Code will also contain the admin keypair to allow the user who scans it to manage the ByzCoin
