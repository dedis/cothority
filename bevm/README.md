# Solidity smart contracts on Byzcoin



## The Byzcoin Virtual Machine contract

We call BVM the standard [EVM](https://en.wikipedia.org/wiki/Ethereum#Virtual_Machine)  with modified parameters running inside the bvmContract. 

The bvmContract is a Byzcoin contract that allows execution of arbitrary solidity code on the Byzcoin ledger. 
 
 The contract supports the following instructions :  

- `Spawn` Instantiate a new ledger with a bvm
- `Invoke:display` display the balance of a given Ethereum address 
- `Invoke:credit` credits an Ethereum address with 5 eth by default
- `Invoke:transaction` sends a transaction to the ledger containing an Ethereum transaction that is then applied to the bvm 



Interacting with the bvm is made through standard Byzcoin transactions. The Ethereum transactions are wrapped inside a Byzcoin transaction, then sent to the BVM running in the bvmContract.

 
## Display and Credit

Display and credit instructions will take as only parameter the Ethereum address in byte format. Display will show the remaining credit of that address, and credit will credit it 5 eth. This was chosen arbitrarily and should be enough for any practical purpose. 

## Transaction

To execute a transaction such as deploying a contract or interacting with an existing contract you will need to sign the transaction with a private key containing enough ether to pay for the execution of the transaction. You will have to credit an address before the next steps to avoid an out of gas error.

#### Gas parameters

You can set custom gasLimit and gasPrice in each transaction or use the `transactionGasParameters` function.

#### Abi & bytecode

You will need both the bytecode (to deploy the contract) and the abi (to interact with it) of your smart contract. Use the `getSC` function or hardcode them directly. 

### Deploy a new contract

Create a contract creation Ethereum transaction using the `NewContractCreation` function carrying your contract bytecode. Then use the `signAndMarshalTx` function before adding the signed transaction to the arguments of a Byzcoin transaction and sending that transaction to the Byzcoin ledger.  

### Interact with an existing contract

Create an Ethereum transaction using the `NewTransaction` function of the types packages, containing : 

• The contract address, derived with the `CreateAddress` function of the crypto package if you don't have the address

• Data containing the method selector (with the abi) and the arguments of said method using the `abiMethodPack` function  

then `signAndMarshalTx` and send to Byzcoin as above.

## Memory abstraction layers 

![Memory Model](https://github.com/dedis/student_18_hugo_verex/public/images/bvmMemory.svg)

## Ethereum State

Defined as 
```golang
type ES struct {
	DbBuf []byte
	RootHash common.Hash
}
```

defined by the general state and the last root commit hash.
To save the root hash : 

```golang
es.RootHash := sdb.Commit
```

where sdb is the state.stateDb. To save the database 

```golang
es.DbBuf, err := memdb.Dump()
```

after having commited the memory database
```golang
err = db.Database().TrieDB().Commit(es.RootHash, true)
		if err != nil {
			return nil, nil, err
		}
```

To get the different databases, simply use the `getDB` function in `params.go`


## Files

The following files are in this directory:

- `bvmContract.go` defines the Byzcoin contract that interacts with the Ethereum Virtual Machine
- `database.go` redefines the Ethereum database functions to be compatible with Byzcoin
- `params.go` defines the parameter of the BVM
- `keys.go` helper methods for Ethereum key management 
- `service.go` only serves to register the contract with ByzCoin. If you
want to give more power to your service, be sure to look at the
[../service](service example).
- `proto.go` has the definitions that will be translated into protobuf

