# el - the CLI to EventLogs

Here are some examples of how to use el.

## Create a new EventLog, saving the config

```
$ el create -roster roster.toml
```

The `roster.toml` file is a list of servers what form the cothority that will
log the events. After running `run_conode.sh local 3` for example, the file `public.toml`
will have the 3 conodes in it. For a larger production deployment, you will construct
the `roster.toml` file by collecting the `public.toml` files from each of the servers.

The event log config info (the skipchain ID and the private key for the owner
Darc) are stored in the local config directory (~/.config/el).

To see the confige you just made, use `el show`, which in fact shows all the configs you have available.

You can choose a config with the `-config #` command, where `#` is the number of the config
from `el show`.

Because config files are named after their skipchain ID in ~/.config/el, you can copy
them to another server if you need to. *Remember* that there is the Darc owner's private key inside
it, and anyone who has the file can evolve the access control on the EventLog.

TODO: The behavior of init is wrong. It needs to take as input a config
for an existing skipchain and then create as output a new owner Darc on that
skipchain. (The skipchain config will come from the Omniledger tool.)

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

```
$ el search -follow -config 2 -topic Topic -from 12:00 -for 1h
```

The `-follow` flag does a normal search, then starts a subscription
on the search in order to see new events as they arrive.

## Evolution (delegation) of access control

Not implemented yet.