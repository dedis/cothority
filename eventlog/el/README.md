Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Applications](https://github.com/dedis/cothority/blob/master/doc/Applications.md) ::
[EventLog](https://github.com/dedis/cothority/blob/master/eventlog/README.md) ::
el

# el - the CLI to EventLogs

Here are some examples of how to use el.

## Make a new key pair and creating an event log

Using the `el` tool, you can create a key pair:

```
$ el key
```

The public key is printed on stdout. The private one is stored in the `el`
configuration directory. To use a custom configuration directory use the
`-config $dir`. You will give the public key to the ByzCoin administrator who
will use the "bcadmin darc rule" command to give your private key the right to
make new event logs (add "spawn:eventlog" and "invoke:eventlog.log" rules to a
Darc). We will not cover how to configure ByzCoin, more information can be
found in the [bcadmin documentation](../../byzcoin/bcadmin/README.md).

The ByzCoin admin will give you a ByzCoin config file, which you will use with
the -bc argument, or you can set the BC environment variable to the name of the
ByzCoin config file. He/she will also give you a DarcID to use.

Assuming ByzCoin is configured with the correct permissions, you can now make
the event log like this:

```
$ el create -bc $file -darc $darcID -sign $key
```

A new event log will be spawned, and the event log ID will be printed. Set the
EL environment variable to communicate it to future calls to the `el` program.
The $key variable is the key which you created using `el key`.

## Logging

```
$ el log -topic Topic -content "The log message" -sign $key
```

The above command creates a log entry. If `-topic` is not set, it defaults to
the empty string. If `-content` is not set, `el log` defaults to reading one
line at a time from stdin and logging those with the given `-topic`.

An interesting test that logs 100 messages, one every .1 second, so
that you can see the messages arriving over the course of several
block creation epochs:

```
$ seq 100 | (while read i; do echo $i; sleep .1; done) | ./el log
```

## Searching

```
$ el search -topic Topic -from 12:00 -to 13:00 -count 5
$ el search -topic Topic -from 12:00 -for 1h
```

The exit code tells you if the search was truncated or not.

If `-topic` is not set, it defaults to the empty string. If you give
`-from`, then you must not give `-to`.

## OpenID authentication (needs to be updated)

If the Darc that controls access to the eventlog has the form
"proxy:$pubkey:$user", then `el` will need to use the
[Authentication Proxy](../../authprox/README.md) in order to get signatures.

The process looks like this:

```
# Enroll the external provider with all the Auth Proxies
$ apadmin add --roster ../../conode/public.toml --issuer https://oauth.dedis.ch/dex
External provider enrolled. Use identities of this form:
	 proxy:969168fa299693ee27d4b6f4d58b5be58fbddbbd769abe1d6a703822a0299804:user@example.com

# Add a Darc rule of that form for both spawning and invoking
$ bcadmin add spawn:eventlog -identity proxy:969168fa299693ee27d4b6f4d58b5be58fbddbbd769abe1d6a703822a0299804:user@example.com
$ bcadmin add invoke:eventlog -identity proxy:969168fa299693ee27d4b6f4d58b5be58fbddbbd769abe1d6a703822a0299804:user@example.com

# Login to the OpenID provider in order to get the authentication token saved into el's config.
$ el login
Opening this URL in your browser:
	 https://oauth.dedis.ch/dex/auth?client_id=dedis&redirect_uri=urn%3Aietf%3Awg%3Aoauth%3A2.0%3Aoob&response_type=code&scope=offline_access+openid+email&state=none
Enter the access code now: pks4ocibylrm67ht5r4jhd2d2
Login information saved into /Users/jallen/Library/Application Support/el/data/openid.cfg

# Now you can send transactions, which instead of being signed with a local private
# key, are signed with the shares of the private key stored in the Authentication Proxies.
$ el create
$ el log -content Test
```

If you want to stop using OpenID authentication, you need to remove the
`$HOME/Library/Application Support/el/data/openid.cfg` file, and then set the `-private`
CLI flag, or `PRIVATE_KEY` environment variable.
