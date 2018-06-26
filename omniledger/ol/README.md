# ol - the CLI to configure OmniLedger permissions

## Create a new OmniLedger, saving the config

```
$ ol create -roster roster.toml
```

The `roster.toml` file is a list of servers what form the cothority that will
maintain the ledger. After running `run_conode.sh local 3` for example, the file
`public.toml` will have the 3 conodes in it. For a larger production deployment,
you will construct the `roster.toml` file by collecting the `public.toml` files
from each of the servers.

The OmniLedger config info (the skipchain ID and the roster for the cothority)
are stored in the local config directory (~/.config/ol or ~/Library/Application
Support/ol) and the filename is printed on stdout. The OmniLedger config file
will be used by other tools to know where to send their transactions.

The secret key is saved in a file named after the public key. It must not be
shared!

To see the config you just made, use `ol show -ol $file`.

## Granting access to contracts

The user who wants to use OmniLedger generates a private key and shares the
public key with you, the OmniLedger admin. You grant access to a given contract
for instructions signed by the given secret key like this:

```
$ ol add -ol $file spawn:eventlog -pub ed25519:dd6419b01b49e3ffd18696c93884dc244b4688d95f55d6c2a4639f2b0ce40710
```

Using the OmniLedger config file you give them and their private key to sign transactions,
they will now be able to use their application to send transactions.

## Environmnet variables

You can set the environment variable OL to the config file for the OmniLedger
you are currently working with. (Client apps should follow this same standard.)