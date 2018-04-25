Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/BuildingBlocks.md) ::
Omniledger

# OmniLedger

This implementation of Omniledger has its goal to implement the protocol
described in the [Omniledger Paper](https://eprint.iacr.org/2017/406.pdf).
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
[Omniledger Paper](https://eprint.iacr.org/2017/406.pdf).

# Structure Definitions

Following is an overview of the most important structures defined in Omniledger
and how they can be described using protobuf. For each protobuf description we
give a short overview of the different fields and how they work together.

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
	// Signature is calculated over the protobuf representation of [Rules, Version, Description]
	// and needs to be created by an Owner from the previous valid Darc.
	bytes Signature = 5;
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
can use logical operators to specify the rule.  For exmple, the expression
"darc:a & ed25519:b | ed25519:c" means that "darc:a" and at least one of
"ed25519:b" and "ed25519:c" must sign. For more information please see the
expression package.

## Transactions

A block in omniledger contains zero or more transactions. Each transaction

A transaction in Omniledger has the following format:

```
message Transaction {
  bytes Key = 1;
  bytes Kind = 2;
  bytes Value = 3;
  enum Operation {
    ADD = 0;
    UPDATE = 1;
    DELETE = 2;
  }
  Operation Operation = 4;
  DarcSignature Signature = 5;
  boolean Valid = 6;
}
```

The *Key* is created similar to the way Ethereum creates its addresses and is
always 64 bytes long. The lower 32 bytes are filled with the BaseID of the
Darc that allows the Key/Kind/Value triplet to be stored in omniledger.
The upper 32 bytes are a nonce that needs to be unique for unique triplets,
but that doesn't need to be monotonic or increasing.

The *Kind* is the hash of the precompiled smart contract that will be executed
to verify the triplet is valid. Some examples of such contracts are:

- create a new Darc
- Administer omniledger:
  - Add or remove nodes
  - Change the time between two blocks
- Onchain-secrets:
  - Create a write request
  - Create a read request
- Evoting:
  - Setting up a new election
  - Casting a vote
  - Requesting mix
  - Requesting decryption
- PoP:
  - Create a new party

The *Value* is any slice of bytes that will be interpreted correctly by the
smart contract.

The *Operation* defines whether this is a new triplet, an update to an existing one
or whether it deletes the triplet. The smart contract has to take into account
the different Operation requested.

The *Signature* is created on the darc-action with regard to the Transaction
and needs to be verifiable by all nodes.

The *Valid* field is filled in by the leader to indicate whether this transaction
will be included in the system state or not. Every node needs to make sure that
valid transactions are correct and that invalid transactions are indeed incorrect.

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

During verification, the verifier then can do the following:

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

# Usage and Comments

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
