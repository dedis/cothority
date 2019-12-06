Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
[BEvm](https://github.com/dedis/cothority/blob/master/bevm/README.md) ::
bevmadmin

# bevmadmin - CLI tool to manage BEvm instances

For the details on all the options and arguments, invoke the tool using the `--help` option.

## Create a new BEvm instance
Assuming ByzCoin config and key files in the current directory (see [bcadmin](https://github.com/dedis/cothority/blob/master/byzcoin/bcadmin/README.md) for details):
```bash
bevmadmin --config . spawn --bc bc-<ByzCoinID>.cfg
```

## Delete an existing BEvm instance
```bash
bevmadmin --config . delete --bc bc-<ByzCoinID>.cfg --bevm-id <BEvm instance ID>
```
