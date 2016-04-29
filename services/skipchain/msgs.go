package skipchain

import (
	"github.com/dedis/cothority/lib/network"
	"github.com/satori/go.uuid"
)

func init() {
	var msgs = []interface{}{
		// Propose
		&ProposeSkipBlock{},
		&ProposeSkipBlockData{},
		&ProposeSkipBlockRoster{},
		&ProposedSkipBlockReply{},
		&ProposedSkipBlockReplyData{},
		&ProposedSkipBlockReplyRoster{},
		&SetChildrenSkipBlock{},
		&SetChildrenSkipBlockReply{},
		// Propagation
		&PropagateSkipBlockRoster{},
		&PropagateSkipBlockData{},
		// Requests for data
		&GetUpdateChain{},
		&GetUpdateChainReply{},
		// Data-structures
		&ForwardSignature{},
		&SkipBlockFix{},
		&SkipBlockCommon{},
		&SkipBlockData{},
		&SkipBlockRoster{},
	}
	for _, m := range msgs {
		network.RegisterMessageType(m)
	}
}

type VerifierID uuid.UUID
type RosterID uuid.UUID

var (
	VerifyShard     = VerifierID(uuid.NewV5(uuid.NamespaceURL, "Shard"))
	VerifyTUF       = VerifierID(uuid.NewV5(uuid.NamespaceURL, "TUF"))
	VerifySSH       = VerifierID(uuid.NewV5(uuid.NamespaceURL, "SSH-ks"))
	VerifyConiks    = VerifierID(uuid.NewV5(uuid.NamespaceURL, "Coniks"))
	VerifyTimeVault = VerifierID(uuid.NewV5(uuid.NamespaceURL, "TimeVault"))
	VerifyNone      = VerifierID(uuid.Nil)
)

// This file holds all messages that can be sent to the SkipChain,
// both from the outside and between instances of this service

// External calls

// RequestNewBlock - Requests a new skipblock to be appended to
// the given SkipBlock. If the given SkipBlock has Index 0 (which
// is invalid), a new SkipChain will be created.
// The AppId will be used to call the corresponding verification-
// routines who will have to sign off on the new Tree.
type ProposeSkipBlock struct {
	Latest   SkipBlockID
	Proposed SkipBlock
}

type ProposeSkipBlockData struct {
	Latest   SkipBlockID
	Proposed *SkipBlockData
}

type ProposeSkipBlockRoster struct {
	Latest   SkipBlockID
	Proposed *SkipBlockRoster
}

// ProposedSkipBlockReply - returns the signed SkipBlock with updated backlinks
type ProposedSkipBlockReply struct {
	Previous SkipBlock
	Latest   SkipBlock
}

type ProposedSkipBlockReplyData struct {
	Parent   *SkipBlockRoster
	Previous *SkipBlockData
	Latest   *SkipBlockData
}

type ProposedSkipBlockReplyRoster struct {
	Previous *SkipBlockRoster
	Latest   *SkipBlockRoster
}

// GetUpdateChain - the client sends the hash of the last known
// Skipblock and will get back a list of all necessary SkipBlocks
// to get to the latest.
type GetUpdateChain struct {
	Latest SkipBlockID
}

// GetUpdateChainRet - returns the shortest chain to the current SkipBlock,
// starting from the SkipBlock the client sent
type GetUpdateChainReply struct {
	UpdateData   []*SkipBlockData
	UpdateRoster []*SkipBlockRoster
}

// SetChildrenSkipBlock adds a child-SkipBlock to a parent SkipBlock
type SetChildrenSkipBlock struct {
	Parent SkipBlockID
	Child  SkipBlockID
}

// SetChildrenSkipBlockReply is the reply from SetChildrenSkipBlock. Only one
// of ChildData and ChildRoster will be non-nil
type SetChildrenSkipBlockReply struct {
	Parent      *SkipBlockRoster
	ChildData   *SkipBlockData
	ChildRoster *SkipBlockRoster
}

// GetChildrenSkipList - if the SkipList doesn't exist yet, creates the
// Genesis-block of that SkipList.
// It returns a 'GetUpdateChainReply' with the chain from the first to
// the last SkipBlock.
type GetChildrenSkipList struct {
	Current    SkipBlock
	VerifierId VerifierID
}

// Internal calls

// PropagateSkipBlock sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlockData struct {
	SkipBlock *SkipBlockData
}

type PropagateSkipBlockRoster struct {
	SkipBlock *SkipBlockRoster
}

// ForwardSignature asks this responsible for a SkipChain to sign off
// a new ForwardLink. This will probably be sent to all members of any
// SkipChain-definition at time 'n'
type ForwardSignature struct {
	ToUpdate SkipBlockID
	Latest   SkipBlock
}
