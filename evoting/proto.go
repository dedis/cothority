package evoting

import (
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/kyber/v3"
)

// PROTOSTART
// package evoting_api;
// type :skipchain.SkipBlockID:bytes
// type :kyber.Point:bytes
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "EvotingAPI";

// Reconstruct message.
type Reconstruct struct {
	// ID of the election skipchain.
	ID skipchain.SkipBlockID
}

// ReconstructReply message.
type ReconstructReply struct {
	// Points are the decrypted plaintexts.
	Points []kyber.Point
	// Eventual additional points - only if more than 9 candidates
	AdditionalPoints []AddPoints
}

// AddPoints if maxChoices > 9
type AddPoints struct {
	// The additional Points
	AdditionalPoints []kyber.Point
}

// Ping message.
type Ping struct {
	// Nonce can be any integer.
	Nonce uint32
}
