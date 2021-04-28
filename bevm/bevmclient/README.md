Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
[BEvm](https://github.com/dedis/cothority/blob/main/bevm/README.md) ::
bevmclient

# bevmclient - CLI tool to deploy and interact with BEvm contracts

For the details on all the options and arguments, invoke the tool using the `--help` option.

## Creating a BEvm account
Assuming ByzCoin config and key files in the current directory (see [bcadmin](https://github.com/dedis/cothority/blob/main/byzcoin/bcadmin/README.md) for details):
```bash
bevmclient createAccount --accountName <MyAccount>
```

## Crediting a BEvm account
```bash
bevmclient --config . creditAccount --bc bc-<ByzCoinID>.cfg --bevmID <BEvm instance ID> --accountName <MyAccount> <amount>
```

## Retrieving the balance of a BEvm account
```bash
bevmclient --config . getAccountBalance --bc bc-<ByzCoinID>.cfg --bevmID <BEvm instance ID> --accountName <MyAccount>
```

## Deploying a BEvm contract
```bash
bevmclient --config . deployContract --bc bc-<ByzCoinID>.cfg --bevmID <BEvm instance ID> --accountName <MyAccount> --contractName <MyContract> <ABI file> <bytecode file> [<arg>...]
```

## Executing a transaction on a deployed BEvm contract instance
```bash
bevmclient --config . transaction --bc bc-<ByzCoinID>.cfg --bevmID <BEvm instance ID> --accountName <MyAccount> --contractName <MyContract> <transaction name> [<arg>...]
```

## Executing a view method on a deployed BEvm contract instance
```bash
bevmclient --config . call --bc bc-<ByzCoinID>.cfg --bevmID <BEvm instance ID> --accountName <MyAccount> --contractName <MyContract> <view method name> [<arg>...]
```
