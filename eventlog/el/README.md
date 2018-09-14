# el - the CLI to EventLogs

Here are some examples of how to use el.

## Make a new key pair

```
$ el create -keys
```

The keys are printed on the stdout. You will give the public key to the
ByzCoin administrator to use with the "ol add" command to give your
private key the right to make new event logs.

```
$ PRIVATE_KEY=$priv el create -ol $file
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



