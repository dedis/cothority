Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/BuildingBlocks.md) ::
OmniLedger

# OmniLedger

This implementation of OmniLedger has its goal to implement the protocol
described in the [OmniLedger Paper](https://eprint.iacr.org/2017/406.pdf).
As the paper is only describing the network interaction and very few of the
details of how the transactions themselves are handled, we will include
them as seem fit.

This document describes the part of omniledger that are implemented and how to
use them. It should grow over time as more parts of the system are implemented.

## Overview

Broadly speaking, omniledger will implement:

1. multiple transactions per block
2. queuing of transactions at each node and periodical creation of a new
block by the leader
3. allow for verification functions that apply to different kinds of data
4. view-change in case the leader fails
5. sharding of the nodes
6. inter-shard transactions

1-4 are issues that should've been included in skipchains for a long time, but
never got in. Only 5-6 are 'real' omniledger improvements as described in the
[OmniLedger Paper](https://eprint.iacr.org/2017/406.pdf).

# Structure Definitions

Following is an overview of the most important structures defined in OmniLedger
and how they can be described using protobuf. For each protobuf description we
give a short overview of the different fields and how they work together.

## Skipchain Block

Whenever OmniLedger stores a new Skipchain Block, the header will only contain
hashes, while the clientTransactions will be stored in the body. This allows
for a reduced proof size.

Block header:
- Merkle tree root of the global state
- Hash of all clientTransactions in this block
- Hash of all stateChanges resulting from the clientTransactions

Block body:
- List of all clientTransactions

## Smart Contracts in OmniLedger

Previous name was _Precompiled Smart Contracts_, but looking at how we want
it to work, we decided to call it simply a set of Contracts. A contract defines
how to interpret the methods sent by the client. It is identified by the
contractID which is a string pointing to a given contract.

Contracts receive as an input a list of coins that are available to them. As
an output, a contract needs to give the new list of coins that is available.

After all contracts have been run, the leftover coins are given to the leader as
a mining reward.

Input arguments:
- pointer to database for read-access
- Instruction from the client
- key/value pairs of coins available

Output arguments:
- one StateChange (might be empty)
- updated key/value pairs of coins still available
- error that will abort the clientTransaction if it is non-zero. No global
state will be changed if any of the contracts returns non-zero.

## From Client to the Collection

In OmniLedger we define the following path from client instructions to
global state changes:

* _Instruction_ is one of Spawn, Invoke or Delete that is called upon an
existing object
* _ClientTransaction_ is a set of instructions sent by a client
* _StateChange_ is calculated at the leader and verified by every node. It
contains the new key/contractID/value triplets to create/update/delete.

A block in omniledger contains zero or more OmniLedgerTransactions. Every
one of these transactions can be valid or not and will be marked as such by
the leader. Every node has to verify whether it accepts or refuses the
decisions made by the leader.

### Authentication and Coins

Current authentications support darc-signatures, later authentications will also
support use of coins. It is the contracts' responsibility to verify the
authentication and that enough coins are available.

### Instruction

An instruction is created by a client. It has the following format:

```
// Instruction holds only one of Spawn, Invoke, or Delete
message Instruction {
	// ObjectID holds the id of the existing object that can spawn new objects.
	ObjectID ObjectID = 1;
	// Nonce is monotonically increasing with regard to the darc in the objectID
	// and used to prevent replay attacks.
	// The client has to track which is the current nonce of a darc-ID.
	bytes Nonce = 2;
	// Index and length prevent a leader from censoring specific instructions from
	// a client and still keep the other instructions valid.
	// Index is relative to the beginning of the clientTransaction.
	int32 Index = 3;
	// Length is the total number of instructions in this clientTransaction
	int32 Length = 4;
	// Spawn creates a new object
	Spawn spawn = 5;
	// Invoke calls a method of an existing object
	Invoke invoke = 6;
	// Delete removes the given object
	Delete delete = 7;
	// Signatures that can be verified using the darc defined by the objectID.
	repeated DarcSignature Signatures = 8;
}

// ObjectID points to an object that holds the state of a contract.
message ObjectID {
	// DarcID points to the darc controlling access to this object
	DarcID DarcID = 1;
	// InstanceID is taken from the Instruction.Nonce when the Spawn instruction is
	// sent.
	bytes InstanceID = 2;
}

// Spawn is called upon an existing object that will spawn a new object.
message Spawn {
	// ContractID represents the kind of contract that needs to be spawn.
	string ContractID = 1;
	// args holds all data necessary to authenticate and spawn the new object.
	repeated Argument args = 2;
}

// Invoke calls a method of an existing object which will update its internal
// state.
message Invoke {
	// Command is object specific and interpreted by the object.
	string Command = 1;
	// args holds all data necessary to authenticate and spawn the new object.
	repeat Argument args = 2;
}

// Delete removes the object.
message Delete {
}

// Argument is a name/value pair that will be passed to the object.
message Argument {
	// Name can be any name recognized by the object.
	string Name = 1;
	// Value must be binary marshalled
	bytes Value = 2;
}
```

### ClientTransaction

If a client needs a set of instructions to be applied atomically by omniledger,
it can send more than one instruction in a ClientTransaction. This structure
has the following format:

```
message ClientTransaction{
	repeated Instruction Instructions = 1;
}
```

### StateChange

Once the leader receives the ClientTransactions, it will send the individual
instructions to the corresponding contracts and/or objects. Each call to a
contract/object will return 0 or more StateChanges that define how to update the
state of the collection.
OmniLedger will take care that the following instruction/StateChanges are
respected. *This might be too restrictive*:
- Spawn: only Create-Actions
- Invoke: only Update-Action on the invoked object
- Delete: only Delete-Action on the invoked object

```
message StateChange{
	// StateAction can be any of Create, Update, Delete
	StateAction StateAction = 1;
	// ObjectID is the identifier of the key
	bytes ObjectID = 2;
	// ContractID points to the contract that can interpret the value
	bytes ContractID = 3;
	// Value is the data needed by the contract
	bytes Value = 4;
}
```

The *ObjectID* is a random key chosen by OmniLedger and must correspond to
further Instructions sent by the client.

## Proof

The proof in omniledger proves the absence or the presence of a key in the state
of the given skipchain. If the key is present, the proof also contains the kind
and the value of that key.

To verify the proof, all the verifier needs is the skipchain-ID of where the
key is supposed to be stored. The proof has three parts:

1. _InclusionProof_ proofs the presence or absence of the key. In case of
the key being present, the value is included in the proof.
2. _Latest_ is used to verify the merkle tree root used in the collection-proof
is stored in the latest skipblock.
3. _Links_ proves that the latest skipblock is part of the skipchain.

So the protobuf-definition of a proof is the following:

```
message Proof {
	// InclusionProof is the deserialized InclusionProof
	collection.Proof InclusionProof = 1;
	// Providing the latest skipblock to retrieve the Merkle tree root.
	skipchain.SkipBlock Latest = 2;
	// Proving the path to the latest skipblock. The first ForwardLink has an
	// empty-sliced `From` and the genesis-block in `To`, together with the
	// roster of the genesis-block in the `NewRoster`.
	repeated skipchain.ForwardLink Links = 3;
}

message skipchain.SkipBlock{
  // Many omitted fields
  bytes Data = 9;
  // Other omitted fields
}

message skipchain.ForwardLink{
  // From - where this forward link comes from
  bytes From = 1;
  // To - where this forward link points to
  bytes To = 2;
  // NewRoster is only set to non-nil if the From block has a
  // different roster from the To-block.
  onet.Roster NewRoster = 3;
  // Signature is calculated on the
  // sha256(From.Hash()|To.Hash()|NewRoster)
  // In the case that NewRoster is nil, the signature is
  // calculated on the sha256(From.Hash()|To.Hash())
  byzcoinx.FinalSignature Signature = 4;
}

```

During verification, the verifier then can do the following to make sure that the
key/value pair returned is valid:

1. Verify the inclusion proof of the key in the merkle tree root of the collection.
This is described in the [colleciton](#collection) section.
2. Verify the merkle tree root in the InclusionProof is the same as the one
given in the latest skipblock
3. Verify the Links are a valid chain from the genesis block to the latest block.
The first forward link points to the genesis block to give the roster to the
verifier, so the verifier only needs the skipchain-id and doesn't need to have
the genesis block.

## Collection

The collection is a Merkle-tree based data structure to securely and
verifiably store key / value associations on untrusted nodes. The library
in this package focuses on ease of use and flexibility, allowing to easily
develop applications ranging from simple client-server storage to fully
distributed and decentralized ledgers with minimal bootstrapping time.

Our collection used is a library that has been
[developed for a PhD project](collection/README.md) and
can do much more than simple Merkle-trees. Depending on the future direction
of the project, it might be replaced by a simpler Merkle-tree implementation.

## Darc

Package darc in most of our projects we need some kind of access control to
protect resources. Instead of having a simple password or public key for
authentication, we want to have access control that can be: evolved with a
threshold number of keys be delegated. So instead of having a fixed list of
identities that are allowed to access a resource, the goal is to have an
evolving description of who is allowed or not to access a certain resource.

A darc has the following format:

```
message Darc {
	// Version should be monotonically increasing over the evolution of a Darc.
	uint64 Version = 1;
	// Description is a free-form field that can hold any data as required by the user.
	// Darc itself will never depend on any of the data in here.
	bytes Description = 2;
	// BaseID is the ID of the first darc of this Series
	bytes BaseID = 3;
	// Rules map an action to an expression.
	Rules Rules = 4;
	// Path represents the path to get up to information to be able to
	// verify this signature. These justify the right of the signer to push
	// a new Darc. These are ordered from the oldest to the newest, i.e.
	// Path[0] should be the base Darc. This field is optional unless
	// offline verification is needed.
	repeated Darc Path
	// PathDigest is the digest that represent the path above.
	bytes PathDigest = 5
	// Signature is calculated on the Request-representation of the darc.
	// It needs to be created by identities that have the "_evolve" action
	// from the previous valid Darc.
	repeated bytes Signature = 6;
}

message Rule {
	map<string, bytes> Rules = 1;
}
```

The primary type is a darc. Which contains a set of rules that what type of
permission are granted for any identity. A darc can be updated by performing an
evolution.  That is, the identities that have the "evolve" permission in the
old darc can creates a signature that signs off the new darc. Evolutions can be
performed any number of times, which creates a chain of darcs, also known as a
path. A path can be verified by starting at the oldest darc (also known as the
base darc), walking down the path and verifying the signature at every step.

As mentioned before, it is possible to perform delegation. For example, instead
of giving the "evolve" permission to (public key) identities, we can give it to
other darcs. For example, suppose the newest darc in some path, let's called it
darc_A, has the "evolve" permission set to true for another darc---darc_B, then
darc_B is allowed to evolve the path.

Of course, we do not want to have static rules that allows only a single
signer.  Our darc implementation supports an expression language where the user
can use logical operators to specify the rule.  For example, the expression
"darc:a & ed25519:b | ed25519:c" means that "darc:a" and at least one of
"ed25519:b" and "ed25519:c" must sign. For more information please see the
expression package.

# Usage and Comments

## Contract Examples

Examples of contracts and some of their methods are:

- Darc:
  - create a new Darc
	- update a darc
- OmniLedger Configuration
  - create new configuration
  - Add or remove nodes
  - Change the block interval time
- Onchain-secrets write request:
  - create a write request
- Onchain-secrets read request:
  - create a read request
- Onchain-secrets reencryption request:
  - create a reencryption request
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

## Transaction Queue and Block Generation

This part of the document describes the technical details of the design and
implementation of transaction queue and block generation for OmniLedger. The
assumption is that the leader will not fail. We will implement view-change in
the future, starting by eliminating stop-failure and then byzantine-failure.
Further, we assume there exists a maximum block size of B bytes. Transaction
Queue A transaction is similar to what is defined above, namely a key/kind/value
triplet and a signature of the requester (client). However, for bookkeeping
purposes, we add a private field: "status". A status can be one of three
states: "New" | "Verified" | "Submitted". The transaction request message is
the same as the Transaction struct above, e.g.

```
message TransactionRequest {
  Transaction Transaction = 1;
}
```

TransactionRequest can be sent to any conode. The client should send it to
multiple conodes if it suspects that some of the conode might fail or are
malicious. On the conode, it will store all transactions that it received, in a
queue in memory, with the initial state being "New". We use a slice with a mutex
as the implementation for the queue. If the total size of the queue exceeds B
bytes (we may need to adjust this to support a large backlog), then the conode
responds to the client with a failure message, otherwise a success message. The
client should not see the success message as an indication that the transaction
is included in the block, but that the transaction is received and may be
accepted into a block. We do not attempt to check whether the transaction is
valid at this point because the conode’s darc database might not be up-to-date,
for example if it just came back online.  

### Block Generation

The poll method is inspired by the
beta synchroniser, where the leader sends a message, e.g.

```
message PollTxRequest{
  bytes LatestBlockID = 1;
}
```

down the communication tree, and then every
node will respond with a type

```
message PollTxResponse {
  repeated Transaction Txs = 1;
}
```

The transactions are combined on the subleader nodes.

However, before sending the `PollTxResponse` message, the conodes must check that
the state of omniledger does include the transactions in the latest block
given by the id in `PollTxRequest`. If the state is not
up-to-date, then the nodes must do an update to ensure it is. Then, the nodes
verify `Transaction.Signature` to make sure that all transaction in their queue are
valid. The valid transactions are marked as "Verified" and the bad transactions
are dropped and a message is printed to the audit log. Finally, the transactions
with the "Verified" flag are sent to the leader in the `PollTxResponse` message.
These transactions are marked as "Submitted". The `PollTxResponse` message should
not be larger than B bytes.

Upon receiving all the `PollTxResponse` message, the leader does the following:
- remove duplicates
- verify signature
- sort the transactions in a random but deterministic way
- go through the list of transactions, and for each transaction mark if it
applies correctly to the state updated with previous valid transactions

Then the leader creates the block and then sends it to the conodes to cosign, e.g.

```
message BlockProposal {
  Data Data = 1;
}
```

The conodes run the same checks and need to make sure that the transactions are
in the same state as marked by the leader. If this is the case, they sign the
hash of the proposed block.

The new block, with a collective signature, is propagated back to all nodes.
Then every node updates their queue and removes the transactions that are in the
new block. For the transactions that were not added to the new block, they need
to be moved to the front of the queue and marked as "New" because the state
of omniledger may have changed and the old transactions may become invalid. All the
"Verified" transactions must also be changed back to "New".

### Additional blocks

What we described above is how to generate a single block, how do we run it
multiple times? A simple solution would be for the leader to send a
`PollTxRequest` after every new block is generated. However, it results in a lot
of wasted messages if there are very little or no transactions. We can attempt
to implement the simplest technique first and then try to optimise it later. For
example, a slightly better version would be to add some delay when the blocks
are getting smaller. But this is only a heuristic because the leader does not
know how many transactions are in the queues of the non-root nodes.

Another option would be that each node sends _one_ message to the leader if it
has not-included transactions. This could happen when:
- the queue has been empty and a first transaction comes in
- a new block has been accepted, but not all transaction of the queue are in
that block
This would be halfway between only depending on the leader and sending _all_
transactions to the leader.
