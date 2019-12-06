Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
[BEvm](https://github.com/dedis/cothority/blob/master/bevm/README.md) ::
bevmclient

# bevmclient - CLI tool to deploy and interact with BEvm contracts

For the details on all the options and arguments, invoke the tool using the `--help` option.

## Create a BEvm account
Assuming ByzCoin config and key files in the current directory (see [bcadmin](https://github.com/dedis/cothority/blob/master/byzcoin/bcadmin/README.md) for details):
```bash
bevmclient create_account --account-name <MyAccount>
```

## Credit a BEvm account
```bash
bevmclient --config . credit_account --bc bc-<ByzCoinID>.cfg --bevm-id <BEvm instance ID> --account-name <MyAccount> <amount>
```

## Retrieve the balance of a BEvm account
```bash
bevmclient --config . get_account_balance --bc bc-<ByzCoinID>.cfg --bevm-id <BEvm instance ID> --account-name <MyAccount>
```

## Deploy a BEvm contract
```bash
bevmclient --config . deploy_contract --bc bc-<ByzCoinID>.cfg --bevm-id <BEvm instance ID> --account-name <MyAccount> --conjtract-name <MyContract> <ABI file> <bytecode file> [<arg>...]
```

## Execute a transaction on a deployed BEvm contract instance
```bash
bevmclient --config . transaction --bc bc-<ByzCoinID>.cfg --bevm-id <BEvm instance ID> --account-name <MyAccount> --contract-name <MyContract> <transaction name> [<arg>...]
```

## Execute a view method on a deployed BEvm contract instance
```bash
bevmclient --config . call --bc bc-<ByzCoinID>.cfg --bevm-id <BEvm instance ID> --account-name <MyAccount> --contract-name <MyContract> <view method name> [<arg>...]
```
