Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/doc/BuildingBlocks.md) ::
[ByzCoin](README.md) ::
Data Structures

# Data Structures

This document gives an overview of the basic data structures used in ByzCoin.
Here is a summary:

- A [ClientTransaction](#clienttransaction) is sent by a client to one or more
nodes and holds one or more Instructions.
- An [Instruction](#instruction) is a basic building block that will be executed
in ByzCoin. It has either a `Spawn`, `Invoke`, or a `Delete` command. Once
accepted, every instruction creates zero or more `StateChanges`.
- [StateChange](#statechange)s are collected and define how the global state will
change.
- [Darc](#darc)s control access to executing Instructions. The signers of an
Instruction must satisfy one of the rules in the associated Darc.
- A [Proof](#proof) shows to a client that his instruction has been accepted by
ByzCoin.

## ClientTransaction

If a client needs a set of instructions to be applied atomically by ByzCoin,
it can send more than one instruction in a `ClientTransaction`. This structure
has the following format:

```protobuf
message ClientTransaction {
	repeated Instruction Instructions = 1;
}
```

## Instruction

An instruction is created by a client. It has the following format:

```protobuf
// Instruction holds only one of Spawn, Invoke, or Delete
message Instruction {
  // InstanceID is either the instance that can spawn a new instance, or the instance
  // that will be invoked or deleted.
  required bytes instanceid = 1;
  // Nonce is monotonically increasing with regard to the Darc controlling
  // access to the instance. It is used to prevent replay attacks.
  // The client has to track what the next nonce should be for a given Darc.
  required bytes nonce = 2;
  // Index and length prevent a leader from censoring specific instructions from
  // a client and still keep the other instructions valid.
  // Index is relative to the beginning of the clientTransaction.
  required sint32 index = 3;
  // Length is the total number of instructions in this clientTransaction
  required sint32 length = 4;
  // Spawn creates a new instance.
  optional Spawn spawn = 5;
  // Invoke calls a method of an existing instance.
  optional Invoke invoke = 6;
  // Delete removes the given instance.
  optional Delete delete = 7;
  // Signatures that are verified using the Darc controlling access to the instance.
  repeated darc.Signature signatures = 8;
}

// Spawn is called upon an existing instance that will spawn a new instance.
message Spawn {
  // ContractID represents the kind of contract that needs to be spawn.
  required string contractid = 1;
  // Args holds all data necessary to spawn the new instance.
  repeated Argument args = 2;
}

// Invoke calls a method of an existing instance which will update its internal
// state.
message Invoke {
  // Command is interpreted by the contract.
  required string command = 1;
  // Args holds all data necessary for the successful execution of the command.
  repeated Argument args = 2;
}

// Delete removes the instance. The contract might enforce conditions that
// must be true before a Delete is executed.
message Delete {
}
```

An `InstanceID` is a series of 32 bytes. The spawn implementation in the contract
chooses the new instance ID, and after that it is the client's responsibility to
track it in order to be able to send in Invoke instructions on it later.

## StateChange

Once the leader receives a `ClientTransaction`, it will send the individual
instructions to the corresponding contracts and/or objects. Each call to a
contract/object will return 0 or more `StateChange` elements that define how to update the
state of the trie.

ByzCoin will take care that instructions respect the following rules.
*This might be too restrictive*:
- Spawn: only Create-Actions
- Invoke: only Update-Action on the invoked object
- Delete: only Delete-Action on the invoked object

```protobuf
// StateChange is one new state that will be applied to the trie.
message StateChange {
  // StateAction can be any of Create, Update, Remove
  required sint32 stateaction = 1;
  // InstanceID of the state to change
  required bytes instanceid = 2;
  // ContractID points to the contract that can interpret the value
  required bytes contractid = 3;
  // Value is the data needed by the contract
  required bytes value = 4;
  // DarcID is the Darc controlling access to this key.
  required bytes darcid = 5;
  // Version is the instance version for this particular state change
  required uint64 version = 6;
}
```

## Proof

The proof in ByzCoin proves the absence or the presence of a key in the state
of the given ByzCoin. If the key is present, the proof also contains the value
of the key, as well as the contract that wrote it, and the DarcID of the Darc
that controls access to it.

To verify the proof, all the verifiers need the skipchain-ID of where the
key is supposed to be stored. The proof has three parts:

1. *InclusionProof* proves the presence or absence of the key. In case of
the key being present, the value is included in the proof.
2. *Latest* is used to verify the Merkle tree root used in the proof is stored
   in the latest skipblock.
3. *Links* proves that the latest skipblock is part of the skipchain.

So the protobuf-definition of a proof is the following:

```protobuf
message Proof {
	// InclusionProof is the deserialized InclusionProof
	trie.Proof InclusionProof = 1;
	// Providing the latest skipblock to retrieve the Merkle tree root.
	skipchain.SkipBlock Latest = 2;
	// Proving the path to the latest skipblock. The first ForwardLink has an
	// empty-sliced `From` and the genesis-block in `To`, together with the
	// roster of the genesis-block in the `NewRoster`.
	repeated skipchain.ForwardLink Links = 3;
}

message skipchain.SkipBlock {
	// Many omitted fields
	bytes data = 8;
	// Other omitted fields
}

message skipchain.ForwardLink {
	// From - where this forward link comes from
	bytes from = 1;
	// To - where this forward link points to
	bytes to = 2;
	// NewRoster is only set to non-nil if the From block has a
	// different roster from the To-block.
	onet.Roster newRoster = 3;
	// Signature is calculated on the
	// sha256(From.Hash()|To.Hash()|NewRoster)
	// In the case that NewRoster is nil, the signature is
	// calculated on the sha256(From.Hash()|To.Hash())
	ByzcoinSig signature = 4;
}

message ByzcoinSig {
	required bytes msg = 1;
	required bytes sig = 2;
}
```

During verification, the verifier can then do the following to make sure the
key/value pair returned is valid:

1. Verify the inclusion proof of the key in the Merkle tree root of the trie.
This is described in the [trie](trie/README.md) package.
2. Verify the Merkle tree root in the `InclusionProof` is the same as the one
given in the latest skipblock
3. Verify the Links are a valid chain from the genesis block to the latest block.
The first forward link points to the genesis block to give the roster to the
verifier, so the verifier only needs the skipchain-id and doesn't need to have
the genesis block.

## Darc

A darc has the following format:

```protobuf
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

The primary type is a darc, which contains a set of rules that tell what type of
permissions are granted for each identity. A darc can be updated by performing an
evolution. The identities that have the "evolve" permission in the
old darc can creates a signature that approves the new darc. Evolutions can be
performed any number of times, which creates a chain of darcs, also known as a
path. A path can be verified by starting at the oldest darc (also known as the
base darc), walking down the path and verifying the signature at every step.

As mentioned before, it is possible to perform delegation. For example, instead
of giving the "evolve" permission to (public key) identities, we can give it to
other darcs. For example, suppose the newest darc in some path, let's called it
`darc_A`, has the "evolve" permission set to true for another darc `darc_B`, then
`darc_B` is allowed to evolve the path.

Of course, we do not want to have static rules that allows only a single
signer.  Our darc implementation supports an expression language where the user
can use logical operators to specify the rule.  For example, the expression
`darc:a & ed25519:b | ed25519:c` means that `darc:a` and at least one of
`ed25519:b` and `ed25519:c` must sign. For more information please see the
expression package.
