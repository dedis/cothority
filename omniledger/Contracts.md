Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/doc/BuildingBlocks.md) ::
[OmniLedger](README.md) ::
Contracts and Instances

# Contracts and Instances

A contract in omniledger is similar to a smart contract in Ethereum, except that
it is pre-compiled in the code and all nodes need to have the same version of
the contract available in order to reach consensus.

A contract can spawn new instances that are tied to another contract type. All
instances are stored in the global state of omniledger. Every instance points
to exactly one contract. An easy interpretation is to think of a contract as
a class and the instance as an object instantiated from that class. Or, in Go,
as the instance being the values of the `struct` and the contract being all
methods defined on that `struct`.

## Authorizations

Authorizations are handled using Darcs. Each Darc has a set of rules that define
a pair of action / expression that need to be fulfilled to execute any instruction
on the instances governed by that Darc.

A Darc is always stored with the following `InstanceID`:

`[32]byte{DarcBaseID}[32]byte{0}`

If a Darc is updated (evolved), it will overwrite the existing Darc. All
instances spawned by that Darc (except for other Darcs) will share the same
first half of the `InstanceID`. The second half is pseudo-randomly chosen by the
contract having spawned this new instance. The current implementation takes the
hash of the instruction for the `subID` (the second part of the `InstanceID`).

Given the following instruction sent by a client (some fields are omitted for
clarity):

- `InstanceID`: `[32]byte{GenesisDarc}[32]byte{0x01}`
- `Invoke`:
  - `Command`: `Update`
  - `Args`: `{"Roster": NewRoster}`
- `Signatures`: `[Sig1]`

omniledger will do the following:

1. find the Darc instance by looking at the first 32 bytes of the `InstanceID` given in
the instruction, here the `GenesisDarc`, and adding 32 x 0x00 bytes
2. create a `DarcRequest` using the `InstanceID` and the `Args`
3. verify the request corresponds to the expression of the `Invoke_Update` rule
in the Darc instance found in 1.

## Contract Arguments

A contract is always pre-compiled into every node and has the following
method signature:

```go
type OmniLedgerContract func(coll CollectionView, tx Instruction, inCoins []Coin) (sc []StateChange, outCoins []Coin, err error)
```

Input:
- `coll` is a read-only reference to the collection representing the global state
of all instances
- `tx` is the instruction sent by the client, which also holds the `InstanceID`
pointing to the data the contract should work on
- `inCoins` is mostly ignored for the moment, but can be used to pass around
coins between different instructions

Output:
- `sc` is the slice of stateChanges the contract wants to apply to the global
state. They will only be applied if all instructions in the `ClientTransaction`
are valid, else they will be discarded
- `outCoins` can be used to pass around coins
- `err` if not nil, the contract indicates it failed, and all instructions in that
`ClientTransaction` will be discarded

The contract itself has access to all elements of the collection, but will mainly
work on the data pointed to by the `tx Instruction` given as a parameter. It is
not allowed to change the collection by itself, only by creating one or more
`StateChange`s that create/update/delete instances in the global state.

The `StateChange`s are applied between all instructions to a temporary copy of
the collection, and only committed if all instructions are successful, else all
`StateChange`s from this `ClientTransaction` will be discarded.

If there is more than one `ClientTransaction` in a block, the contracts called
in the second `ClientTransaction` will see all changes applied from the first
`ClientTransaction.`

## Instance Structure

Every instance in omniledger is stored with the following information in the
global state:

- `InstanceID` is a globally unique identifier of that instance, composed of:
  - `DarcID`, defining the access rights to that instance
  - `SubID`, which is randomly chosen, currently implemented as taking the hash
  of the instruction, so that the client can now what instance he will create
  when `Spawn`ing an instance. The special `SubID` of `0` indicates the Darc
  responsible for all the instances starting with the same `DarcID`
- `ContractID` points to the contract that will be called if that instance
receives an instruction from the client
- `Data` is interpreted by the contract and can change over time

## Interaction between Instructions and Instances

Every instruction sent by a client indicates the `InstanceID` it is sent to.
Omniledger will start by verifying the authorization as described above, then
use the `InstanceID` to look up the responsible contract for this instance and
then send the instruction to that contract. A client can call an instance with
one of the following three basic instructions:

- `Spawn` - will ask the instance to create a new instance. The client indicates the
requested new contract-type and the arguments. Currently only `Darc` instances can
spawn new instances.
- `Invoke` - sends a method and its arguments to the instance
- `Delete` - requests to delete that instance

# Existing Contracts

In the omniledger service, the following contracts are pre-defined:

- `GenesisReference` - points to the genesis configuration
- `Config` - holds the configuration of omniledger
- `Darc` - defines the access control

To extend omniledger, you will have to create a new service that defines new
contracts that will have to be registered with omniledger. An example is
[EventLog](../../eventlog) that defines a contract.

## Genesis Configuration

The special `InstanceID` with 64 x 0x00 bytes is the genesis configuration
pointer that has as the data the `DarcID` of the genesis Darc. This instance
is unique as it defines the basic running configuration of omniledger. The
Darc it points to will delegate authorizations to spawn new instances to
other Darcs, who can themselves delegate further.

The following two contracts can only be instantiated once in the whole system:

- `GenesisReference`, which has the `InstanceID` of 64 x 0x00 and points to the
genesis Darc
- `Config`, which defines the basic configuration of omniledger:
  - `Roster` is the list of all nodes participating in the consensus

### Spawn

The `Config` contract can spawn new Darcs or any other type of instances that
are available to omniledger.

### Invoke

- `Config_Update` - stores a new configuration

## Darc Contract

The most basic contract in omniledger is the `Darc` contract that defines the
access rules for all clients. When creating a new omniledger blockchain, a
genesis Darc instance is created, which indicates what instructions need which
signatures to be accepted.

### Spawn

When the client sends a spawn instruction to a Darc contract, he asks the instance
to create a new instance with the given ContractID, which can be different from
the Darc itself. The client must be able to authenticate against a
`Spawn_contractid` rule defined in the Darc instance.

### Invoke

The only method that a client can invoke on a Darc instance is `Evolve`, which
asks omniledger to store a new version of the Darc in the global state.

### Delete

When a Darc instance receives a `Delete` instruction, it will be removed from the
global state.

## Possible future contracts

Here is a short list of possible future contracts that are imaginable. But
your coding skills set the limits:

- OmniLedger Configuration
  - create new configuration
  - Add or remove nodes
  - Change the block interval time
- Onchain-secrets
  - write request: create a write request
  - read request: create a read request
  - reencryption request: create a reencryption request
- Evoting:
  - Creating a new election
  - Casting a vote
  - Requesting mix
  - Requesting decryption
- PoP:
  - Create a new party
  - Adding attendees
	- Finalizing the party
- PoPCoin:
  - Creating a popcoin source
- PoPCoinAccount:
  - Creating an account
	- Transfer coins from one account to another
