package collection

import "crypto/sha256"

// PROTOSTART
// type :children:Children
// type :dump:Dump
// type :step:Step
// package collection;
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "CollectionProto";

// dump

type dump struct {
	Key      []byte
	Values   [][]byte
	Children children
	Label    [sha256.Size]byte
}

type children struct {
	Left  [sha256.Size]byte
	Right [sha256.Size]byte
}

// step

type step struct {
	Left  dump
	Right dump
}

// Proof

// Proof is an object representing the proof of presence or absence of a given key in a collection.
type Proof struct {
	// Key is the key that this proof is representing
	Key []byte
	// Root is the root node
	Root dump
	// Steps are the steps to go from root to key
	Steps      []step
	collection *Collection
}
