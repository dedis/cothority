Navigation: [DEDIS](https://github.com/dedis/doc/tree/master/README.md) ::
[Cothority](https://github.com/dedis/cothority/tree/master/README.md) ::
[Building Blocks](https://github.com/dedis/cothority/tree/master/BuildingBlocks.md) ::
[OmniLedger](README.md) ::
Data Structures

# Data Structures

This document gives an overview of the basic data structures used in omniledger:

- [ClientTransaction](#ClientTransaction) is sent by a client to one or more
nodes and holds one or more Instructions:
- [Instruction](#Instruction) is a basic building block that will be executed
in omniledger. It has either a `Spawn`, `Invoke`, or a `Delete` command. Once
accepted, every instruction creates zero or more `StateChanges`:
- [StateChange](#StateChange) are collected and define how the global state will
change.
- [Darc](#Darc) protect the Instructions and proof that the instructions have
been created by an authorized user.
- [Proof](#Proof) shows to a client that his instruction has been accepted by
omniledger.

## ClientTransaction

If a client needs a set of instructions to be applied atomically by omniledger,
it can send more than one instruction in a ClientTransaction. This structure
has the following format:

```
message ClientTransaction{
	repeated Instruction Instructions = 1;
}
```

## Instruction

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

## StateChange

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

## Darc

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
