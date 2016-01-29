package cosi

import (
	"github.com/dedis/cothority/lib/sda"
)

// RoundCosi is the implementation of a vanilla CoSi protocol
type RoundCosi struct {
	Node *sda.Node
}

const RoundCosiType = "cosi"

// Register the RoundCosi New function
func init() {
	RegisterRoundFactory(RoundCosiType,
		func(node *sda.Node) Round {
			return NewRoundCosi(node)
		})
}

// NewRoundCosi returns a freshly new generated RoundCosi
func NewRoundCosi(node *sda.Node) *RoundCosi {
	return &RoundCosi{
		Node: node,
	}
}

// Announcement phase:
func (rc *RoundCosi) Announcement(*AnnouncementMessage, []*AnnouncementMessage) error {
	return nil
}

// Commitmemt phase:
func (rc *RoundCosi) Commitment([]*CommitmentMessage, *CommitmentMessage) error {
	return nil
}

// Challenge phase:
func (rc *RoundCosi) Challenge(*ChallengeMessage, []*ChallengeMessage) error {
	return nil
}

// Response phase:
func (rc *RoundCosi) Response([]*ResponseMessage, *ResponseMessage) error {
	return nil
}
