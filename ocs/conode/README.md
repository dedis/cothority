# Conode

This is a copy of the original

https://github.com/dedis/cothority/conode

for use in the onchain-secrets package.

## Using run_conode.sh

To start a set of local conodes that store documents, simply run:

```bash
./run_conode.sh local 3 2
```

The number `3` indicates how many nodes should be started, and the number `2`
indicates the debug-level: `0` is silent, and `5` is very verbose.

To stop the running nodes, use

```bash
pkill -f conode
```
