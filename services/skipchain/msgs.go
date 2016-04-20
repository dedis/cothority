package skipchain

import (
	"github.com/dedis/crypto/abstract"
)

// This file holds all messages that can be sent to the SkipChain,
// both from the outside and between instances of this service

// External calls - there are two modes:
// 1 - active: all nodes are always up and reachable
// 2 - polling: nodes can be down and behind NATs

// ActiveAdd - adds a new skipblock to the chain. Previous may be nil
// if you want to create a new SkipChain.
// The forward-link of 'Previous' has to be signed by the cothority
type ActiveAdd struct {
	Previous *SkipBlock
	New      *SkipBlock
}

// ActiveAddRet - returns the signed SkipBlock with updated backlinks
type ActiveAddRet struct {
	*SkipBlock
}

// PollPreCommit - sends commits for future challenges. At least
// maxHeight + 1 commits need to be sent in advance
type PollPreCommit struct {
	Commits []*abstract.Point
}

// PollPropose - sends a proposition for a new SkipBlock and commits. It
// will return PollChallenge. If New is nil, it will return a PollChallenge
// if one is waiting or a ErrorRet{nil} if the queue is empty.
type PollPropose struct {
	New *SkipBlock
}

// PollChallenge - returns all SkipBlocks that need updated ForwardLinks
// and the corresponding challenges
type PollChallenge struct {
	SkipBlocks []*SkipBlock
	Challenges []*abstract.Secret
}

// PollResponse - sends the response for the requested SkipBlocks.
// The commits sent will only be used for future SkipBlocks
type PollResponse struct {
	SkipBlocks []*SkipBlock
	Responses  []*abstract.Secret
	Commits    []*abstract.Point
}

// GetUpdateChain - the client sends the last known SkipBlock and will
// get the shortest chain to the current SkipBlock
type GetUpdateChain struct {
	*SkipBlock
}

// GetUpdateChainRet - returns the shortest chain to the current SkipBlock,
// starting from the SkipBlock the client sent
type GetUpdateChainRet struct {
	Update []*SkipBlock
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
