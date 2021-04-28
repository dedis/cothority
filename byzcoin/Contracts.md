Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/main/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/main/doc/BuildingBlocks.md) ::
[ByzCoin](README.md) ::
Contracts and Instances

# Contracts and Instances

A contract in ByzCoin is similar to a smart contract in Ethereum, except that
it is pre-compiled in the code and all nodes need to have the same version of
the contract available in order to reach consensus. Or, if there are variations
in the implementation of the contract, the output of the various implementations
must be equal.

A contract can spawn new instances that are tied to another contract type. All
instances are stored in the global state of ByzCoin. Every instance points
to exactly one contract. An easy interpretation is to think of a contract as
a class and the instance as an object instantiated from that class.

## Authorizations

Authorizations are handled using Darcs. Each Darc has a set of rules that define
a pair of action / expression that need to be fulfilled to execute any instruction
on the instances governed by that Darc.

A Darc is always stored with an `InstanceID` equal to the Darc's base ID.
If a Darc is updated (evolved), it will overwrite the existing Darc.

Given the following instruction sent by a client (some fields are omitted for
clarity):

- `InstanceID`: `[32]byte{GenesisDarcID}`
- `Invoke`:
  - `Command`: `Update`
  - `Args`: `{"Roster": NewRoster}`
- `Signatures`: `[Sig1]`

ByzCoin will do the following:

1. find the Darc instance using the `InstanceID`.
2. create a `DarcRequest` using the `InstanceID` and the `Args`
3. verify the request corresponds to the expression of the `invoke:update` rule
in the Darc instance found in 1.

## Contract Arguments

A contract is a collection of methods on a structure. Together these methods
implement the `byzcoin.Contract` type, which is:

```go
type Contract interface {
	// Verify returns nil if the instruction is valid with regard to the signature.
	VerifyInstruction(ReadOnlyStateTrie, Instruction, []byte) error
	// Spawn is used to spawn new instances
	Spawn(ReadOnlyStateTrie, Instruction, []Coin) ([]StateChange, []Coin, error)
	// Invoke only modifies existing instances
	Invoke(ReadOnlyStateTrie, Instruction, []Coin) ([]StateChange, []Coin, error)
	// Delete removes the current instance
	Delete(ReadOnlyStateTrie, Instruction, []Coin) ([]StateChange, []Coin, error)
}
```

ByzCoin needs a "factory" function that can turn the contents of an instance
into a pointer to structure which implements the byzcoin.Contract interface.
This factory function is of type `byzcoin.ContractFn`:

```go
// ContractFn is the type signature of the instance factory functions which can be
// registered with the ByzCoin service.
type ContractFn func(in []byte) (Contract, error)
```

The factory function is registered with ByzCoin via the `RegisterContract` function
during server startup.

The simplest way to make a contract is to define a structure that embeds
`byzcoin.BasicContract`. This structure already implements all of the functions
a contract needs to implement, with a default `VerifyInstruction` and
other methods which return a "not implemented" error. Attach methods to your
new structure as needed (i.e. `Spawn` and `Invoke`, but not `Delete` if your
contract does not support deleting instances) in order to
implement the behaviours that your contract supports.

When ByzCoin needs to call your contract, it will look up the current
value of the instance and pass it to your factory function. In the case
of a spawn, it passes `nil`, because the instance does not exist yet; your
factory function should return a pointer to a zero-value structure in
this case.

ByzCoin will then call the appropriate methods on this structure. It starts
by calling `VerifyInstruction`, and if that does not return an error, it will
call `Spawn`, `Invoke`, or `Delete`, depending on the instruction.

These methods take the following input:
- A read-only reference to the trie representing the global state of
all instances.
- The instruction sent by the client, which also holds the `InstanceID`
pointing to the data the contract should work on.
- A list of coins given as input to this instruction.

They create the following output:
- A slice of state changes the contract wants to apply to the global
state. They will only be applied if all instructions in the `ClientTransaction`
are valid, else they will be discarded.
- A list of coins remaining as output from this instruction, and will
be passed as input to the next instruction.
- An error. If it is not `nil`, the contract indicates it failed, and all instructions in that
`ClientTransaction` will be discarded.

The contract itself has access to all elements of the trie, but will mainly
work on the data pointed to by the instruction given as a parameter. It is
not allowed to change the trie by itself, only by creating one or more
`StateChange`s that create/update/delete instances in the global state.

The `StateChange`s are applied between all instructions to a temporary copy of
the trie, and only committed if all instructions are successful, else all
`StateChange`s from this `ClientTransaction` will be discarded.

If there are more than one `ClientTransaction`s in a block, the contracts called
in the second `ClientTransaction` will see all changes applied from the first
`ClientTransaction.`

## Instance Structure

Every instance in ByzCoin is stored with the following information in the
global state:

- `InstanceID` is a globally unique identifier of that instance, composed
of 32 bytes.
- `Version` is the version number of this update to the instance, starting from 0.
- `ContractID` points to the contract that will be called if that instance
receives an instruction from the client
- `Data` is interpreted by the contract and can change over time
- `DarcID` of the Darc that controls access to this instance.

## Interaction between Instructions and Instances

Every instruction sent by a client indicates the `InstanceID` it is sent to.
ByzCoin will start by verifying the authorization as described above, then
use the `InstanceID` to look up the responsible contract for this instance and
then send the instruction to that contract. A client can call an instance with
one of the following three basic instructions:

- `Spawn` - will ask the instance to create a new instance. The client indicates the
requested new contract-type and the arguments. Currently only `Darc` instances can
spawn new instances.
- `Invoke` - sends a method and its arguments to the instance
- `Delete` - requests to delete that instance

# Existing Contracts

In the ByzCoin service, the following contracts are pre-defined:

- `Config` - holds the configuration of ByzCoin
- `SecureDarc` - defines the access control

To extend ByzCoin, you will have to create a new service that defines new
contracts that will have to be registered with ByzCoin. An example is
[EventLog](../../eventlog) that defines a contract.

## Genesis Configuration

The special `InstanceID` with 32 0 bytes is the genesis configuration
pointer that has as the data the `DarcID` of the genesis Darc. This instance
is unique as it defines the basic running configuration of ByzCoin. The
Darc it points to will delegate authorizations to spawn new instances to
other Darcs, who can themselves delegate further.

The config holds the interval for the blocks, and also the current roster
of nodes that collectively witness the transactions.

### Spawn

The `Config` contract can spawn new Darcs or any other type of instances that
are available to ByzCoin.

### Invoke

- `Config_Update` - stores a new configuration

## SecureDarc Contract

The SecureDarc contract that defines the access rules for all clients using the
Darc data structure. When creating a new ByzCoin blockchain, a genesis Darc
instance is created, which indicates what instructions need which signatures to
be accepted.

### Spawn

When the client sends a spawn instruction to a SecureDarc contract, he asks the
instance to create a new instance with the given ContractID, which can be
different from the SecureDarc instance itself. The client must be able to
authenticate against a `spawn:$contractid` rule defined in the SecureDarc
instance. For instance, to call Spawn on a contract ID of "eventlog", you must
send a spawn request to an instance of a SecureDarc contract that includes the
"spawn:eventlog" rule, and the instruction must be signed by one of the keys
mentioned in the rule's expression.

The new instance spawned will have an instance ID equal to the hash of
the Spawn instruction. The client can remember this instance ID in order
to invoke methods on it later.

### Invoke

The first method that a client can invoke on a SecureDarc instance is `evolve`,
which asks ByzCoin to store a new version of the Darc. The rules may be
modified but new rules cannot be added. Use `evolve_unrestricted` to add rules.

### Delete

When a Darc instance receives a `Delete` instruction, it will be removed from
the global state.

### Secure Darc Customization

Sometimes the user needs fine-grained control of all aspects of their access
control policy. For these purposes we encourage the user to write their own
Darc contracts.

Suppose we are in a hierarchical situation where the boss is allowed to do
anything, the managers are allowed to do a certain set of tasks including
spawning user Darcs and the users are allowed to do another set of tasks but
they are not allowed to spawn any new Darcs. The SecureDarc contract already
achieves some of the requirements. If the managers are benign, they would spawn
users with the correct set of rules. Thus the users cannot give themselves
extra permission because they are only authorized to invoke their `evolve`
action. If the boss is benign, then he/she will only spawn managers with the
correct set of rules. What the SecureDarc cannot do is to stop managers from
spawning user Darcs with arbitrary rules. We can prevent this problem by
writing new Darc contracts. A simple solution is to write three contracts:
BossDarc, ManagerDarc and UserDarc. The BossDarc will be the same as
SecureDarc. The ManagerDarc will have additional logic in its Spawn function
which stops it from spawning manager or boss Darcs. Finally, the UserDarc will
not be allowed to spawn any other Darc.

## Possible future contracts

Here is a short list of possible future contracts that are imaginable. But
your coding skills set the limits:

- ByzCoin Configuration
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
