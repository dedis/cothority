package cothority

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/cothority/lib/sign"
)

// This file is the implementation of a round of a Cothority-based protocol.
// This implementation covers the basic,i.e. "vanilla" Cothority round:
//  - Announcement
//  - Commitment
//  - Challenge
//  - Response

// Since cothority is our main focus, we will be using the Round structure
// defined round.go. You will be able to use this protocol cothority with many
// different rounds very easily.

// ProtocolCothority is the main structure holding the round and the sda.Node.
type ProtocolCothority struct {
	// The round we are using now.
	round Round
	// The node that is representing us. Easier to just embed it.
	*sda.Node
}

// Start() will call the announcement function of its inner Round structure. It
// will pass nil as *in* message.
func (pc *ProtocolCothority) Start() error {
	return pc.handleAnnouncement(nil)
}

func (pc *ProtocolCothority) Dispatch(msgs []*sda.SDAData) error {
	panic("Should not happen since ProtocolCothority uses channel registration")
}

// handleAnnouncement will pass the message to the round and send back the
// output.
func (pc *ProtocolCothority) handleAnnouncement(in *sign.AnnouncementMessage) error {
	// create output buffer if we are not leaf
	var out []*sign.AnnouncementMessage
	if !pc.IsLeaf() {
		out = make([]*sign.AnnouncementMessage, len(pc.Children()))
		for i := range out {
			out[i] = &sign.AnnouncementMessage{
				SigningMessage: in.SigningMessage,
			}
		}
	} else {
		out = nil
	}

	// send it to the round
	if err := pc.round.Announcement(in, out); err != nil {
		return err
	}

	// If we are leaf, we should go to commitment
	if pc.IsLeaf() {
		return pc.handleAllCommitment(nil)
	}

	// send the output to children
	var err error
	for i, tn := range pc.Children() {
		// still try to send to everyone
		err = pc.SendTo(tn, out[i])
	}
	return err
}

// handleAllCommitment takes the full set of messages from the children and pass
// it along the round.
func (pc *ProtocolCothority) handleAllCommitment(in []*sign.CommitmentMessage) error {
	// create output message
	var out *sign.CommitmentMessage
	// dispatch it to the round
	if err := pc.round.Commitment(in, out); err != nil {
		return err
	}

	// if we are the root, we need to start the Challenge
	if pc.IsRoot() {
		return pc.handleChallenge(nil)
	}

	// otherwise send it to parent
	return pc.SendTo(pc.Parent(), out)

}

// handleChallenge dispatch the challenge to the round and then dispatch the
// results down the tree.
func (pc *ProtocolCothority) handleChallenge(in *sign.ChallengeMessage) error {
	var out []*sign.ChallengeMessage
	// if we are not a leaf, we  create output buffer
	if !pc.IsLeaf() {
		out := make([]*sign.ChallengeMessage, len(pc.Children()))
	} else {
		out = nil
	}

	// dispatch it to the round
	if err := pc.round.Challenge(in, out); err != nil {
		return err
	}

	// if we are leaf, then go to response
	if pc.IsLeaf() {
		return pc.handleResponse(nil)
	}

	// otherwise send it to children
	var err error
	for i, tn := range pc.Children() {
		err = pc.SendTo(tn, out[i])
	}
	return err
}

// handleResponse brings up the response of each node in the tree to the root.
func (pc *ProtocolCothority) handleResponse(in []*sign.ResponseMessage) error {
	var out *sign.ResponseMessage
	// dispatch it
	if err := pc.round.Response(in, out); err != nil {
		return err
	}

	// send it back to parent
	return pc.SendTo(pc.Parent(), out)
}
