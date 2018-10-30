Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Applications](https://github.com/dedis/cothority/blob/master/doc/Applications.md) ::
[EventLog](https://github.com/dedis/cothority/blob/master/eventlog/README.md) ::
el

# el - the CLI to EventLogs

Here are some examples of how to use el.

## Make a new key pair

Using the `bcadmin` tool, you can create a key pair:

```
$ bcadmin keys
```

The keys are printed on the stdout. You will give the public key to the
ByzCoin administrator to use with the "bcadmin add" command to give your
private key the right to make new event logs.

```
$ PRIVATE_KEY=$priv el create -bc $file
```

The ByzCoin admin will give you a ByzCoin config file, which you will
use with the -bc argument, or you can set the BC environment variable to the
name of the ByzCoin config file. A new event log will be spawned, and the
event log ID will be printed. Set the EL environment variable to communicate
it to future calls to the `el` program.

You need to give the private key from above, using the PRIVATE_KEY environment
variable or the `-priv` argument.

## Logging

```
$ el log -config 2 -topic Topic -content "The log message"
```

Using config #2, log a string to the event log.

If `-topic` is not set, it defaults to the empty string. If `-content`
is not set, `el log` defaults to reading one line at a time from stdin
and logging those with the given `-topic`.

An interesting test that logs 100 messages, one every .1 second, so
that you can see the messages arriving over the course of several
block creation epochs:

```
$ seq 100 | (while read i; do echo $i; sleep .1; done) | ./el log
```

## Searching

```
$ el search -config 2 -topic Topic -from 12:00 -to 13:00 -count 5
$ el search -config 2 -topic Topic -from 12:00 -for 1h
```

The exit code tells you if the search was truncated or not. (TODO: Should
we make the CLI re-search up to N times upon detecting truncation?)

If `-topic` is not set, it defaults to the empty string. If you give
`-for`, then you must not give `-to`. The default for `-from` is 1
hours ago.

## OpenID authentication

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
