package cosi

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/satori/go.uuid"
	"sync"
)

// This file is the implementation of a round of a Cothority-based protocol.
// This Cosi protocol is the simplest version, the "vanilla" version with the
// four rounds.
//  - Announcement
//  - Commitment
//  - Challenge
//  - Response

// Since cothority is our main focus, we will be using the Round structure
// defined round.go. You will be able to use this protocol cothority with many
// different rounds very easily.

// ProtocolCosi is the main structure holding the round and the sda.Node.
type ProtocolCosi struct {
	// The round we are using now.
	round Round
	// The node that is representing us. Easier to just embed it.
	*sda.Node
	// TreeNodeId cached
	treeNodeId uuid.UUID
	// The channel waiting for Announcement message
	announce chan chanAnnouncement
	// the channel waiting for Commitment message
	commit chan chanCommitment
	// the channel waiting for Challenge message
	challenge chan chanChallenge
	// the channel waiting for Response message
	response chan chanResponse
	// the channel that indicates if we are finished or not
	done chan bool
	// temporary buffer of commitment messages
	tempCommitment []*CommitmentMessage
	// lock associated
	tempCommitLock *sync.Mutex
	// temporary buffer of Response messages
	tempResponse []*ResponseMessage
	// lock associated
	tempResponseLock *sync.Mutex
}

// NewProtocolCosi returns a ProtocolCosi with the round set and the
// node set with the right channels.
// If the round = nil, then we will create a default RoundCosi type instead.
// Use this function like this:
// ```
// round := NewRound****()
// fn := func(n *sda.Node) sda.ProtocolInstance {
//      pc := NewProtocolCosi(round,n)
//		return pc
// }
// sda.RegisterNewProtocolName("cothority",fn)
// ```
func NewProtocolCosi(round Round, node *sda.Node) (*ProtocolCosi, error) {
	var err error
	// if no round given
	if round == nil {
		// create the default RoundCosi
		round, err = NewRoundFromType("RoundCosi", node)
		if err != nil {
			// Ouch something went wrong.
			return nil, err
		}
	}
	pc := &ProtocolCosi{
		round:            round,
		Node:             node,
		done:             make(chan bool),
		tempCommitLock:   new(sync.Mutex),
		tempResponseLock: new(sync.Mutex),
	}
	// Register the three channels we want to register and listens on
	// By passing pointer = automatic instantiation
	node.RegisterChannel(&pc.announce)
	node.RegisterChannel(&pc.commit)
	node.RegisterChannel(&pc.challenge)
	node.RegisterChannel(&pc.response)

	// start the routine that listens on these channels
	go pc.listen()
	return pc, err
}

// Start() will call the announcement function of its inner Round structure. It
// will pass nil as *in* message.
func (pc *ProtocolCosi) Start() error {
	return pc.handleAnnouncement(nil)
}

// Dispatch is not used, and already panics because it's DEPRECATED.
func (pc *ProtocolCosi) Dispatch(msgs []*sda.SDAData) error {
	panic("Should not happen since ProtocolCosi uses channel registration")
}

// listen will listen on the four channels we use (i.e. four steps)
func (pc *ProtocolCosi) listen() {
	for {
		var err error
		select {
		case packet := <-pc.announce:
			err = pc.handleAnnouncement(&packet.AnnouncementMessage)
		case packet := <-pc.commit:
			err = pc.handleCommitment(&packet.CommitmentMessage)
		case packet := <-pc.challenge:
			err = pc.handleChallenge(&packet.ChallengeMessage)
		case packet := <-pc.response:
			// Go !
			err = pc.handleResponse(&packet.ResponseMessage)
		case <-pc.done:
			return
		}
		if err != nil {
			dbg.Error("ProtocolCosi -> err treating incoming:", err)
		}
	}
}

// handleAnnouncement will pass the message to the round and send back the
// output. If in == nil, we are root and we start the round.
func (pc *ProtocolCosi) handleAnnouncement(in *AnnouncementMessage) error {
	// create output buffer if we are not leaf
	var out []*AnnouncementMessage
	if !pc.IsLeaf() {
		out = make([]*AnnouncementMessage, len(pc.Children()))
		for i := range out {
			out[i] = &AnnouncementMessage{
				From: pc.treeNodeId,
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
		return pc.handleCommitment(nil)
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
func (pc *ProtocolCosi) handleCommitment(in *CommitmentMessage) error {
	// add to temporary
	pc.tempCommitLock.Lock()
	pc.tempCommitment = append(pc.tempCommitment, in)
	pc.tempCommitLock.Unlock()
	// do we have enough ?
	if len(pc.tempCommitment) < len(pc.Children()) {
		return nil
	}

	// create output message
	var out *CommitmentMessage
	out.From = pc.treeNodeId
	// dispatch it to the round
	if err := pc.round.Commitment(pc.tempCommitment, out); err != nil {
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
func (pc *ProtocolCosi) handleChallenge(in *ChallengeMessage) error {
	var out []*ChallengeMessage
	// if we are not a leaf, we  create output buffer
	if !pc.IsLeaf() {
		out := make([]*ChallengeMessage, len(pc.Children()))
		for i := range out {
			out[i] = &ChallengeMessage{
				From: pc.treeNodeId,
			}
		}
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
func (pc *ProtocolCosi) handleResponse(in *ResponseMessage) error {
	// add to temporary
	pc.tempResponseLock.Lock()
	pc.tempResponse = append(pc.tempResponse, in)
	pc.tempResponseLock.Unlock()
	// do we have enough ?
	if len(pc.tempResponse) < len(pc.Children()) {
		return nil
	}

	var out *ResponseMessage
	out.From = pc.treeNodeId
	// dispatch it
	if err := pc.round.Response(pc.tempResponse, out); err != nil {
		return err
	}

	// send it back to parent
	return pc.SendTo(pc.Parent(), out)
}
