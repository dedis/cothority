# Measuring transactions per second

In this directory is a simulation which uses the coins contract to move
coins from one user to another one at a time.

# Running it

Have two servers of type ICC.S4 up and running. Then run the simulation like
this:

```
$ go build
$ ./simulation -platform mininet coins.toml
```

The first time, you need to do the interactive setup in order to tell it the
server numbers it will be talking to.

# Results

Running `awk -f txsec.awk < test_data/coins.csv` will print an estimate of
transactions per second by calculating `transactions/round_wall_avg`. Be
careful: edits to `coins.toml` can result in the array indices in txsec.awk
being wrong, watch the first line of output to be sure it is calculating what it
should.

Below is a log of some measurements we've taken using ICC.S4 servers
from [IC Cluster](https://icitdocs.epfl.ch/display/clusterdocs/Types+of+Servers).

| Commit | Txn/sec |
|--------|---------|
| 3af2fef52f1230e146cd9da1a0081ac442ea3a6c| 95 |