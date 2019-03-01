package unicore

import (
	"go.dedis.ch/cothority/v3/skipchain"
	"go.dedis.ch/onet/network"
)

func init() {
	network.RegisterMessages(
		&GetStateRequest{}, &GetStateReply{},
	)
}

// PROTOSTART
// type :skipchain.SkipBlockID:bytes
//
// package unicore;
//
// option java_package = "ch.epfl.dedis.lib.proto";
// option java_outer_classname = "UnicoreProto";

// GetStateRequest is the request to get the current state of an executable
type GetStateRequest struct {
	ByzCoinID  skipchain.SkipBlockID
	InstanceID []byte
}

// GetStateReply returns the state of an executable
type GetStateReply struct {
	Value []byte
}
