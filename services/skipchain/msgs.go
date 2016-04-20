package skipchain

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	var msgs = []interface{}{
		&AddSkipBlock{},
		&AddRet{},
	}
	for _, m := range msgs {
		network.RegisterMessageType(m)
	}
}

// This file holds all messages that can be sent to the SkipChain,
// both from the outside and between instances of this service

// External calls

// AddSkipBlock - adds a new skipblock to the chain. Previous may be nil
// if you want to create a new SkipChain.
// The forward-link of 'Previous' has to be signed by the cothority
type AddSkipBlock struct {
	Previous     *SkipBlock
	PreviousTree *sda.TreeMarshal
	New          *SkipBlock
	NewTree      *sda.TreeMarshal
}

// ActiveAddRet - returns the signed SkipBlock with updated backlinks
type AddRet struct {
	*SkipBlock
	Tree *sda.TreeMarshal
}

// GetUpdateChain - the client sends the hash of the last known
// Skipblock and will get back a list of all necessary SkipBlocks
// to get to the latest.
type GetUpdateChain struct {
	hash []byte
}

// GetUpdateChainRet - returns the shortest chain to the current SkipBlock,
// starting from the SkipBlock the client sent
type GetUpdateChainRet struct {
	Update []*SkipBlock
	Tree   []*sda.TreeMarshal
}

// Internal calls

// PropagateSkipBlock sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlock struct {
	*SkipBlock
}

// ForwardSignature asks this responsible for a SkipChain to sign off
// a new ForwardLink. This will probably be sent to all members of any
// SkipChain-definition at time 'n'
type ForwardSignature struct {
	Old    *SkipBlock
	Latest *SkipBlock
}
