Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](../README.md) ::
[Building Blocks](../doc/BuildingBlocks.md) ::
BEvm

# Ethereum Smart Contracts on ByzCoin

The `bevm` ByzCoin contract allows to load and execute Ethereum contracts compiled to bytecode.

*Note:* all of the contracts and APIs in this directory are highly experimental, and
are currently supported on a best-effort basis by https://C4DT.org, who contributed it.
While C4DT and DEDIS continue to experiment with this code, it is not yet bound by our
normal commitments to backwards compatibility within a major release. We expect to stabilise
this new feature in an upcoming major release (v4.x.y, due February 2019).
The `bevm` feature is only available in release binaries for Linux. If you
are running a conode on another kind of server, you will need to add bevm to `../conode/conode.go`
and build locally with `go build`.

## The ByzCoin Virtual Machine contract

We call BEvm the standard [EVM](https://en.wikipedia.org/wiki/Ethereum#Virtual_Machine) running within a ByzCoin contract.

This contract is called the BEvmContract, and it allows the execution of arbitrary Solidity code on the ByzCoin ledger.

The contract implements the following operations:

- `spawn:bevm` Instantiate a new BEvmContract.
- `invoke:bevm.credit` Credit an Ethereum address with the given amount of Ether.
- `invoke:bevm.transaction` Execute the given transaction on the EVM, saving its state within ByzCoin. The transaction can be an Ethereum contract deployment or a method call.
- `delete:bevm` Delete a BEvmContract instance, along with all its state.

Interaction with the BEvm is made through standard ByzCoin transactions. The Ethereum transactions are wrapped inside ByzCoin transactions and sent to the BEvmContract.

To execute a transaction, such as deploying a contract or interacting with an existing contract, the transaction must be signed with a private key associated to an address containing enough Ether to pay for the execution of the transaction; in case the address balance in not sufficient, an Out Of Gas error will result.

"Gas Limit" and "Gas Price" parameters must also be provided when executing a transaction.

## Client API

The following types are defined in `bevm_client.go`:

- `EvmContract` represents an Ethereum contract, and is initialized by `NewEvmContract()` providing the files containing the bytecode and the ABI.
- `EvmAccount` represents an Ethereum user account, and is initialized by `NewEvmAccount()` provoding the private key.
- `Client` represents the main object to interact with the BEvm.

Note that the BEvmContract does not contain a Solidity compiler, and only handles pre-compiled Ethereum contracts.

Before any BEvm operation can be run, a BEvm instance must be created. This is done using `NewBEvm()` and providing a ByzCoin client, a signer and a Darc. If all goes well, `NewBEvm()` returns the instance ID of the newly created BEvmContract instance.

With this, a new client can be initialized using `NewClient()` and providing again a ByzCoin client, a signer and the BEvmContract instance ID received before.

`Client` supports the following methods:

- `Delete()` deletes the BEvm instance associated to the client, along with all the EVM state it references.
- `Deploy()` deploys a new Ethereum contract. Besides the contract, the following arguments must be provided:
    - a gas limit
    - a gas price
    - an amount, credited to the contract address
    - an account executing the contract deployment; this account's address must have enough balance to execute the transaction
    - the contract constructor arguments
- `Transaction()` executes an Ethereum contract method with side effects. Besides the contract, the following arguments must be provided:
    - a gas limit
    - a gas price
    - an amount, credited to the contract address
    - an account executing the contract deployment; this account's address must have enough balance to execute the transaction
    - the method name
    - the method arguments
- `Call()` executes an Ethereum contract view method (without side effects). Besides the contract, the following arguments must be provided:
    - an account executing the contract deployment; executing a view method does not consume any Ether
    - the method name
    - the method arguments
    - a variable to receive the method return value
- `CreditAccount()` credits the provided Ethereum address with the provided amount.
- `GetAccountBalance()` returns the balance of the provided Ethereum address.

## Ethereum state database storage

The EVM state is maintained in several layered structures, the lower-level of which implementing a simple interface (Put(), Get(), Delete(), etc.). The EVM interacts with this interface using keys and values which are abstract to the user, and represented as sequences of bytes.

The BEvmContract implements this interface in order to store the EVM state database within ByzCoin. Two implementations are provided:

- `MemDatabase` keeps all the data in a map stored in memory; it is mostly used for testing purposes.
- `ByzDatabase` stores the data within ByzCoin, splitting each key/value in a separate instance of a very basic ByzCoin "contract", called a BEvmValue. More precisely, the key is embodied by the instance ID of a BEvmValue (BEvmValue IID = sha256(BEvm IID | key), and the value by the BEvmValue's stored value.

The `ByzDatabase` can be accessed either in a read-only mode (using `ClientByzDatabase` or `StateTrieByzDatabase`) when state modification is not needed, such as for the retrieval of an balance or the execution of a view method, or in a read/write mode (using `ServerByzDatabase`) for executing transactions with side effects.

`ClientByzDatabase` retrieves ByzCoin proofs of the BEvmValue instances to obtain the values. It is used by `Client.Call()` and `Client.GetAccountBalance()`.
`StateTrieByzDatabase` retrieves values using directly a read-only State Trie. It is used by the BEvm `attr` functionality (see below).
`ServerByzDatabase` keeps track of the modifications, and returns a set of StateChanges for ByzCoin to apply. It is used by `Client.Delete()`, `Client.Deploy()`, `Client.Transaction()` and `Client.CreditAccount()`.

## BEvm <=> ByzCoin interaction

Besides providing the possibility to run Ethereum contracts "in isolation", BEvm can also interact with ByzCoin contracts in a few ways, described in the following sections.

### DARC attribute verification

EVM contracts can be used to take part in verifying the validity of ByzCoin instructions, using the `attr` expression evaluation facility.
More precisely, DARC rules can be defined using attributes in their expression that point to a particular method in an EVM contract. During the verification of ByzCoin instructions, this EVM method is then executed, and its result used in the evaluation of the DARC rule protecting it. The BEvm attribute is not a ByzCoin identity, so DARC rules must still rely on standard identities such as ed25519 to provide authentication; they can however provide additional conditions to check for the authorization of an instruction.

This feature can be very useful when DARC rules need a logic or access to information that is application-specific, such as checking that the event takes place within a particular timeframe, or that a value belongs to a list of authorized values. The fact that EVM contracts can be deployed dynamically (without needing to recompile and deploy all the cothority conodes) makes this particularly flexible.

To make use of this attribute, a ByzCoin contract must override the `VerifyInstruction()` method and add the BEvm expression evaluator in the expression evaluator map.

The syntax of a DARC BEvm attribute is the following:

```
attr:bevm:<BEvm instance ID>:<EVM contract address>:<method name>
```

The `BEvm instance ID` is the ByzCoin InstanceID of an existing BEvm instance. Similarly, the `EVM contract address` is the Ethereum address of an EVM contract already deployed with that BEvm instance.

The called EVM method must be a _view method_, that is a method which does not modify the Ethereum state. Of course, the contract can provide other methods controlling a state, with which the client interacts using regular BEvm calls.

For example:

```
attr:bevm:1fb5d8a696cd4b344ce224518829973cc894c28d46c2c66d675d3cc968efe489:0x8CdaF0CD259887258Bc13a92C0a6dA92698644C0:isGreater
```
(the mixed-case EVM address above is due to the [Ethereum address checksum](https://github.com/ethereum/EIPs/blob/master/EIPS/eip-55.md), but the parser is not case-sensitive).

The called EVM method receives the following arguments:

* InstanceID targeted by the instruction
* instruction action
* instruction arguments
* ByzCoin protocol version
* ByzCoin skipblock index
* optional extra information provided by the ByzCoin contract implementation (byte slice)

For further details, please look at the examples in `bevm_attr_test.go`.

### Calls from EVM contracts to ByzCoin contracts

EVM contracts running in BEvm are able to call ByzCoin contracts, that is spawn new instances, invoke instance methods, or delete existing instances.

This functionality makes use of [Solidity events](https://solidity.readthedocs.io/en/v0.6.3/contracts.html#events<Paste>), which are structures that contracts can generate during their execution, and which end up in the EVM transaction receipt. In the Ethereum world, these events are frequently used for logging purposes, or to obtain some feedback from transactions.

BEvm makes use of EVM events by defining some particular event signatures. When an EVM transaction has finished its execution, BEvm examines the transaction logs, and if it recognizes events matching the predefined signatures, it will generate new ByzCoin instructions accordingly, which will be processed by ByzCoin. As it requires ByzCoin state changes, the special events are only processed during `Transaction()` BEvm client calls, not `Call()` calls which are read-only.
The special events are the following:

```
    event ByzcoinSpawn (
        bytes32 instanceID,
        string contractID,
        Argument[] args
    );

    event ByzcoinInvoke (
        bytes32 instanceID,
        string contractID,
        string command,
        Argument[] args
    );

    event ByzcoinDelete (
        bytes32 instanceID,
        string contractID,
        Argument[] args
    );
```

EVM contract methods can emit as many of these events as they wish during a single call, which will all be processed sequentially as part of the BEvm call ByzCoin transaction.

In order to protect the execution of these EVM-generated instructions, a new `evm_contract` DARC identity was defined, which has to be in the corresponding DARC rules protecting the ByzCoin contract calls.

The syntax of an `evm_contract` identity is the following:

```
evm_contract:<BEvm instance ID>:<EVM contract address>
```

For example, in order to allow the spawning of `MyContract` by the EVM contract deployed at address 0x11223344 in BEvm InstanceID aabccddeeff, a DARC must contain the rule `spawn:MyContract evm_contract:aabbccddeeff:0x11223344`. As the generated instructions cannot be cryptographically signed, the `evm_contract` signature is just sha256( BEvm InstanceID | contract address ). Since it would be trivial for a malicious client to create an instruction and forge such a signature, the use of `evm_contract` identities is limited to _synthetic_ instructions, that is the ones generated as a consequence of EVM contract execution.

When spawning new ByzCoin instances as requested by an EVM contract, the problem of determining the new InstanceID arises: they must be computable by the client in order to access the instance later on, while also preventing (intentional or not) clashes. In ByzCoin contracts, this is commonly done using `DeriveID("")`, which uses a hash of the Spawn instruction. Unfortunately, in the case of EVM spawns, the Spawn instruction is generated by ByzCoin and not accessible to the client.

To solve this, when generating a synthetic Spawn instruction, an additional `seed` argument is provided to the instruction. This argument is computed as the `DeriveID("")` of the original BEvm instruction (accessible to the client), to which an one-byte counter is appended, in order to handle the case of multiple spawns within the same EVM call. To generate the new InstanceID, the ByzCoin contract's `Spawn()` implementation should then check whether a `seed` argument is provided, and generate the ID in a way that the client can reproduce; an example is provided in the `ComputeNewInstanceID()` function, which hashes a prefix with the provided seed.

Existing ByzCoin contracts will need to be updated to handle this `seed` argument in their `Spawn()` method. In order to prevent replay issues in case such contracts are EVM-spawned before being modified, a whitelist of _EVM-spawnable_ contracts is defined in BEvm and checked when a new Spawn instruction is generated. When contracts are modified, they can safely update the whitelist to allow their creation by the EVM.

For further details, please look at the examples in `bevm_call_byzcoin_test.go`.
