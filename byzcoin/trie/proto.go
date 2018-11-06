package trie

// PROTOSTART
// package trie;
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "TrieProto";

type interiorNode struct {
	Left  []byte
	Right []byte
}

type emptyNode struct {
	Prefix []bool
}

type leafNode struct {
	Prefix []bool
	Key    []byte
	Value  []byte
}

// Proof contains an inclusion/absence proof for a key.
type Proof struct {
	Interiors []interiorNode
	Leaf      leafNode
	Empty     emptyNode
	Nonce     []byte
	noHashKey bool
}
