# el - the CLI to EventLogs

Here are some examples of how to use el.

## Make a new key pair

```
$ el create -keys
```

The keys are printed on the stdout. You will give the public key to the
OmniLedger administrator to use with the "ol add" command to give your
private key the right to make new event logs.

```
$ PRIVATE_KEY=$priv el create -ol $file
```

The OmniLedger admin will give you an OmniLedger config file, which you will
use with the -ol argument. A new event log will be spawned.

You need to give the private key from above, using the PRIVATE_KEY environment
variable.

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