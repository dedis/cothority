// Implementation of a round of a Collective Signing protocol.
package cosi

import (
	"fmt"
	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/satori/go.uuid"
	"sync"
)

// This Cosi protocol is the simplest version, the "vanilla" version with the
// four phases:
//  - Announcement
//  - Commitment
//  - Challenge
//  - Response
// It uses lib/cosi as the main structure for the protocol.

// ProtocolCosi is the main structure holding the round and the sda.Node.
type ProtocolCosi struct {
	// The node that represents us
	*sda.Node
	// TreeNodeId cached
	treeNodeId uuid.UUID
	// the cosi struct we use (since it is a cosi protocol)
	// Public because we will need it from other protocols.
	Cosi *cosi.Cosi
	// the message we want to sign typically given by the Root
	Message []byte
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
	tempCommitment []*CosiCommitment
	// lock associated
	tempCommitLock *sync.Mutex
	// temporary buffer of Response messages
	tempResponse []*CosiResponse
	// lock associated
	tempResponseLock *sync.Mutex
	// hooks related to the various phase of the protocol.
	// XXX NOT DEPLOYED YET / NOT IN USE.
	// announcement hook
	announcementHook AnnouncementHook
	commitmentHook   CommitmentHook
	challengeHook    ChallengeHook
	DoneCallback     func(chal abstract.Secret, response abstract.Secret)
}

// NewProtocolCosi returns a ProtocolCosi with the node set with the right channels.
// Use this function like this:
// ```
// round := NewRound****()
// fn := func(n *sda.Node) sda.ProtocolInstance {
//      pc := NewProtocolCosi(round,n)
//		return pc
// }
// sda.RegisterNewProtocolName("cothority",fn)
// ```
func NewProtocolCosi(node *sda.Node) (*ProtocolCosi, error) {
	var err error
	pc := &ProtocolCosi{
		Cosi:             cosi.NewCosi(node.Suite(), node.Private()),
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

	return pc, err
}

// NewRootProtocolCosi is used by the root to collectively sign this message
// (vanilla version of the protocol where no contributions are done)
func NewRootProtocolCosi(msg []byte, node *sda.Node) (*ProtocolCosi, error) {
	pc, err := NewProtocolCosi(node)
	pc.Message = msg
	return pc, err
}

// Start() will call the announcement function of its inner Round structure. It
// will pass nil as *in* message.
func (pc *ProtocolCosi) Start() error {
	return pc.StartAnnouncement()
}

// Dispatch will listen on the four channels we use (i.e. four steps)
func (pc *ProtocolCosi) Dispatch() error {
	for {
		var err error
		select {
		case packet := <-pc.announce:
			err = pc.handleAnnouncement(&packet.CosiAnnouncement)
		case packet := <-pc.commit:
			err = pc.handleCommitment(&packet.CosiCommitment)
		case packet := <-pc.challenge:
			err = pc.handleChallenge(&packet.CosiChallenge)
		case packet := <-pc.response:
			err = pc.handleResponse(&packet.CosiResponse)
		case <-pc.done:
			return nil
		}
		if err != nil {
			dbg.Error("ProtocolCosi -> err treating incoming:", err)
		}
	}
}

// StartAnnouncement will start a new announcement.
func (pc *ProtocolCosi) StartAnnouncement() error {
	dbg.Lvl3(pc.Node.Name(), "ProtocolCosi.StartAnnouncement (msg=", pc.Message)
	// First check the hook
	if pc.announcementHook != nil {
		return pc.announcementHook(nil)
	}
	// otherwise make the announcement  yourself
	announcement := pc.Cosi.CreateAnnouncement()

	out := &CosiAnnouncement{
		From:         pc.treeNodeId,
		Announcement: announcement,
	}

	return pc.sendAnnouncement(out)
}

type AnnouncementHook func(in *CosiAnnouncement) error

// handleAnnouncement will pass the message to the round and send back the
// output. If in == nil, we are root and we start the round.
func (pc *ProtocolCosi) handleAnnouncement(in *CosiAnnouncement) error {
	dbg.Lvl3("ProtocolCosi.HandleAnnouncement (msg=", pc.Message)
	// If we have a hook on announcement call the hook
	// the hook is responsible to call pc.Cosi.Announce(in)
	if pc.announcementHook != nil {
		return pc.announcementHook(in)
	}

	// Otherwise, call announcement ourself
	announcement := pc.Cosi.Announce(in.Announcement)

	// If we are leaf, we should go to commitment
	if pc.IsLeaf() {
		return pc.StartCommitment()
	}
	out := &CosiAnnouncement{
		From:         pc.treeNodeId,
		Announcement: announcement,
	}

	// send the output to children
	return pc.sendAnnouncement(out)
}

// sendAnnouncement simply send the announcement to every children
func (pc *ProtocolCosi) sendAnnouncement(ann *CosiAnnouncement) error {
	var err error
	for _, tn := range pc.Children() {
		// still try to send to everyone
		err = pc.SendTo(tn, ann)
	}
	return err
}

type CommitmentHook func(in []*CosiCommitment) error

// StartCommitment create a new commitment and send it up (or to the hook)
func (pc *ProtocolCosi) StartCommitment() error {
	// First check the hook
	if pc.commitmentHook != nil {
		return pc.commitmentHook(nil)
	}
	// otherwise make it yourself
	commitment := pc.Cosi.CreateCommitment()
	out := &CosiCommitment{
		Commitment: commitment,
	}

	dbg.Lvl3(pc.Node.Name(), "ProtocolCosi.StartCommitment() Send to", pc.Parent().Id)
	return pc.SendTo(pc.Parent(), out)
}

// handleAllCommitment takes the full set of messages from the children and passes
// it to the parent
func (pc *ProtocolCosi) handleCommitment(in *CosiCommitment) error {
	// add to temporary
	pc.tempCommitLock.Lock()
	pc.tempCommitment = append(pc.tempCommitment, in)
	pc.tempCommitLock.Unlock()
	// do we have enough ?
	// TODO: exception mechanism will be put into another protocol
	if len(pc.tempCommitment) < len(pc.Children()) {
		return nil
	}
	dbg.Lvl3(pc.Node.Name(), "ProtocolCosi.HandleCommitment aggregated")
	// pass it to the hook
	if pc.commitmentHook != nil {
		return pc.commitmentHook(pc.tempCommitment)
	}

	// or make continue the cosi protocol
	commits := make([]*cosi.Commitment, len(pc.tempCommitment))
	secretVar := pc.Node.Suite().Point().Null()
	for i := range pc.tempCommitment {
		secretVar.Add(secretVar, pc.tempCommitment[i].Commitment.Commitment)
		commits[i] = pc.tempCommitment[i].Commitment
	}

	// go to Commit()
	out := pc.Cosi.Commit(commits)
	secretVar.Add(secretVar, pc.Cosi.GetCommitment())
	// if we are the root, we need to start the Challenge
	if pc.IsRoot() {
		return pc.StartChallenge()
	}

	// otherwise send it to parent
	outMsg := &CosiCommitment{
		Commitment: out,
	}
	return pc.SendTo(pc.Parent(), outMsg)
}

type ChallengeHook func(*CosiChallenge) error

// StartChallenge start the challenge phase. Typically called by the Root ;)
func (pc *ProtocolCosi) StartChallenge() error {
	// first check the hook
	/*if pc.challengeHook != nil {*/
	//return pc.challengeHook(nil)
	/*}*/

	if pc.Message == nil {
		return fmt.Errorf("%s StartChallenge() called without message (=%v)", pc.Node.Name(), pc.Message)
	}
	challenge, err := pc.Cosi.CreateChallenge(pc.Message)
	if err != nil {
		return err
	}
	out := &CosiChallenge{
		Challenge: challenge,
	}
	dbg.Lvl3(pc.Node.Name(), "ProtocolCosi.StartChallenge() chal=", fmt.Sprintf("%+v", challenge))
	return pc.sendChallenge(out)

}

// handleChallenge dispatch the challenge to the round and then dispatch the
// results down the tree.
func (pc *ProtocolCosi) handleChallenge(in *CosiChallenge) error {
	// TODO check hook

	dbg.Lvl3(pc.Node.Name(), "ProtocolCosi.HandleChallenge() chal=", fmt.Sprintf("%+v", in.Challenge))
	// else dispatch it to cosi
	challenge := pc.Cosi.Challenge(in.Challenge)

	// if we are leaf, then go to response
	if pc.IsLeaf() {
		return pc.StartResponse()
	}

	// otherwise send it to children
	out := &CosiChallenge{
		Challenge: challenge,
	}
	return pc.sendChallenge(out)
}

// sendChallenge sends the challenge down the tree.
func (pc *ProtocolCosi) sendChallenge(out *CosiChallenge) error {
	var err error
	for _, tn := range pc.Children() {
		err = pc.SendTo(tn, out)
	}
	return err

}

func (pc *ProtocolCosi) StartResponse() error {
	// TODO check the hook
	// else do it yourself
	resp, err := pc.Cosi.CreateResponse()
	if err != nil {
		return err
	}
	out := &CosiResponse{
		Response: resp,
	}
	dbg.Lvl3(pc.Node.Name(), "ProtocolCosi().StartResponse()")
	err = pc.SendTo(pc.Parent(), out)
	pc.Cleanup()
	return err
}

// handleResponse brings up the response of each node in the tree to the root.
func (pc *ProtocolCosi) handleResponse(in *CosiResponse) error {
	// add to temporary
	pc.tempResponseLock.Lock()
	pc.tempResponse = append(pc.tempResponse, in)
	pc.tempResponseLock.Unlock()
	// do we have enough ?
	dbg.Lvl3(pc.Node.Name(), "ProtocolCosi.HandleResponse() has", len(pc.tempResponse), "responses")
	if len(pc.tempResponse) < len(pc.Children()) {
		return nil
	}
	defer pc.Cleanup()

	dbg.Lvl3(pc.Node.Name(), "ProtocolCosi.HandleResponse() aggregated")
	// TODO check the hook

	// else do it yourself
	responses := make([]*cosi.Response, len(pc.tempResponse))
	for i := range pc.tempResponse {
		responses[i] = pc.tempResponse[i].Response
	}
	outResponse, err := pc.Cosi.Response(responses)
	if err != nil {
		return err
	}

	// verify the responses at each level with the aggregate public key of this
	// subtree.
	if err := pc.Cosi.VerifyResponses(pc.TreeNode().PublicAggregateSubTree); err != nil {
		return fmt.Errorf("%s Verifcation of responses failed:%s", pc.Name(), err)
	}

	out := &CosiResponse{
		Response: outResponse,
	}
	// send it back to parent
	if !pc.IsRoot() {
		return pc.SendTo(pc.Parent(), out)
	}
	return nil
}

// Closes the protocol
func (pc *ProtocolCosi) Cleanup() {
	dbg.Lvl3(pc.Entity().First(), "Cleaning up")
	// if callback when finished
	if pc.DoneCallback != nil {
		pc.DoneCallback(pc.Cosi.GetChallenge(), pc.Cosi.GetAggregateResponse())
	}
	close(pc.done)
	pc.Node.Done()

}

// SigningMessage simply set the message to sign for this round
func (pc *ProtocolCosi) SigningMessage(msg []byte) {
	pc.Message = msg
	dbg.Lvl2(pc.Node.Name(), "Root will sign message=", pc.Message)
}

// TODO Still see if it is relevant...
func (pc *ProtocolCosi) RegisterAnnouncementHook(fn AnnouncementHook) {
	pc.announcementHook = fn
}

func (pc *ProtocolCosi) RegisterCommitmentHook(fn CommitmentHook) {
	pc.commitmentHook = fn
}

func (pc *ProtocolCosi) RegisterChallengeHook(fn ChallengeHook) {
	pc.challengeHook = fn
}

func (pc *ProtocolCosi) RegisterDoneCallback(fn func(chal, resp abstract.Secret)) {
	pc.DoneCallback = fn
}
