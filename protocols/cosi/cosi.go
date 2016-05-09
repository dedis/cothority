// Package cosi implements a round of a Collective Signing protocol.
package cosi

import (
	"fmt"
	"sync"

	"github.com/dedis/cothority/lib/cosi"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
)

func init() {
	sda.ProtocolRegisterName("CoSi", NewProtocolCosi)
}

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
	*sda.TreeNodeInstance
	// TreeNodeId cached
	treeNodeID sda.TreeNodeID
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
	tempCommitment []*Commitment
	// lock associated
	tempCommitLock *sync.Mutex
	// temporary buffer of Response messages
	tempResponse []*Response
	// lock associated
	tempResponseLock *sync.Mutex
	DoneCallback     func(chal abstract.Secret, response abstract.Secret)

	// hooks related to the various phase of the protocol.
	// XXX NOT DEPLOYED YET / NOT IN USE.
	// announcement hook
	announcementHook AnnouncementHook
	commitmentHook   CommitmentHook
	challengeHook    ChallengeHook
}

// AnnouncementHook allows for handling what should happen upon an
// announcement
type AnnouncementHook func(in *Announcement) error

// CommitmentHook allows for handling what should happen when a
// commitment is received
type CommitmentHook func(in []*Commitment) error

// ChallengeHook allows for handling what should happen when a
// challenge is received
type ChallengeHook func(*Challenge) error

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
func NewProtocolCosi(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	var err error
	pc := &ProtocolCosi{
		Cosi:             cosi.NewCosi(node.Suite(), node.Private()),
		TreeNodeInstance: node,
		done:             make(chan bool),
		tempCommitLock:   new(sync.Mutex),
		tempResponseLock: new(sync.Mutex),
	}
	// Register the channels we want to register and listens on

	if err := node.RegisterChannel(&pc.announce); err != nil {
		return pc, err
	}
	if err := node.RegisterChannel(&pc.commit); err != nil {
		return pc, err
	}
	if err := node.RegisterChannel(&pc.challenge); err != nil {
		return pc, err
	}
	if err := node.RegisterChannel(&pc.response); err != nil {
		return pc, err
	}

	return pc, err
}

// Start will call the announcement function of its inner Round structure. It
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
			err = pc.handleAnnouncement(&packet.Announcement)
		case packet := <-pc.commit:
			err = pc.handleCommitment(&packet.Commitment)
		case packet := <-pc.challenge:
			err = pc.handleChallenge(&packet.Challenge)
		case packet := <-pc.response:
			err = pc.handleResponse(&packet.Response)
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
	dbg.Lvl3(pc.Name(), "Message:", pc.Message)
	out := &Announcement{
		From:         pc.treeNodeID,
		Announcement: pc.Cosi.CreateAnnouncement(),
	}

	return pc.handleAnnouncement(out)
}

// handleAnnouncement will pass the message to the round and send back the
// output. If in == nil, we are root and we start the round.
func (pc *ProtocolCosi) handleAnnouncement(in *Announcement) error {
	dbg.Lvl3("Message:", pc.Message)
	// If we have a hook on announcement call the hook
	// the hook is responsible to call pc.Cosi.Announce(in)
	if pc.announcementHook != nil {
		return pc.announcementHook(in)
	}

	// Otherwise, call announcement ourself
	announcement := pc.Cosi.Announce(in.Announcement)

	// If we are leaf, we should go to commitment
	if pc.IsLeaf() {
		return pc.handleCommitment(nil)
	}
	out := &Announcement{
		From:         pc.treeNodeID,
		Announcement: announcement,
	}

	// send the output to children
	return pc.sendAnnouncement(out)
}

// sendAnnouncement simply send the announcement to every children
func (pc *ProtocolCosi) sendAnnouncement(ann *Announcement) error {
	var err error
	for _, tn := range pc.Children() {
		// still try to send to everyone
		err = pc.SendTo(tn, ann)
	}
	return err
}

// handleAllCommitment takes the full set of messages from the children and passes
// it to the parent
func (pc *ProtocolCosi) handleCommitment(in *Commitment) error {
	if !pc.IsLeaf() {
		// add to temporary
		pc.tempCommitLock.Lock()
		pc.tempCommitment = append(pc.tempCommitment, in)
		pc.tempCommitLock.Unlock()
		// do we have enough ?
		// TODO: exception mechanism will be put into another protocol
		if len(pc.tempCommitment) < len(pc.Children()) {
			return nil
		}
	}
	dbg.Lvl3(pc.Name(), "aggregated")
	// pass it to the hook
	if pc.commitmentHook != nil {
		return pc.commitmentHook(pc.tempCommitment)
	}

	// or make continue the cosi protocol
	commits := make([]*cosi.Commitment, len(pc.tempCommitment))
	secretVar := pc.Suite().Point().Null()
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
	outMsg := &Commitment{
		Commitment: out,
	}
	return pc.SendTo(pc.Parent(), outMsg)
}

// StartChallenge start the challenge phase. Typically called by the Root ;)
func (pc *ProtocolCosi) StartChallenge() error {
	challenge, err := pc.Cosi.CreateChallenge(pc.Message)
	if err != nil {
		return err
	}
	out := &Challenge{
		Challenge: challenge,
	}
	dbg.Lvl3(pc.Name(), "Starting Chal=", fmt.Sprintf("%+v", challenge), " (message =", string(pc.Message))
	return pc.handleChallenge(out)

}

// VerifySignature verifies if the challenge and the secret (from the response phase) form a
// correct signature for this message using the aggregated public key.
// This is copied from lib/cosi, so that you don't need to include both lib/cosi
// and protocols/cosi
func VerifySignature(suite abstract.Suite, msg []byte, public abstract.Point, challenge, secret abstract.Secret) error {
	return cosi.VerifySignature(suite, msg, public, challenge, secret)
}

// handleChallenge dispatch the challenge to the round and then dispatch the
// results down the tree.
func (pc *ProtocolCosi) handleChallenge(in *Challenge) error {
	// TODO check hook

	dbg.Lvl3(pc.Name(), "chal=", fmt.Sprintf("%+v", in.Challenge))
	// else dispatch it to cosi
	challenge := pc.Cosi.Challenge(in.Challenge)

	// if we are leaf, then go to response
	if pc.IsLeaf() {
		return pc.handleResponse(nil)
	}

	// otherwise send it to children
	out := &Challenge{
		Challenge: challenge,
	}
	return pc.sendChallenge(out)
}

// sendChallenge sends the challenge down the tree.
func (pc *ProtocolCosi) sendChallenge(out *Challenge) error {
	var err error
	for _, tn := range pc.Children() {
		err = pc.SendTo(tn, out)
	}
	return err

}

// handleResponse brings up the response of each node in the tree to the root.
func (pc *ProtocolCosi) handleResponse(in *Response) error {
	if !pc.IsLeaf() {
		// add to temporary
		pc.tempResponseLock.Lock()
		pc.tempResponse = append(pc.tempResponse, in)
		pc.tempResponseLock.Unlock()
		// do we have enough ?
		dbg.Lvl3(pc.Name(), "has", len(pc.tempResponse), "responses")
		if len(pc.tempResponse) < len(pc.Children()) {
			return nil
		}
	}
	defer pc.Cleanup()

	dbg.Lvl3(pc.Name(), "aggregated")
	responses := make([]*cosi.Response, len(pc.tempResponse))
	for i := range pc.tempResponse {
		responses[i] = pc.tempResponse[i].Response
	}
	outResponse, err := pc.Cosi.Response(responses)
	if err != nil {
		return err
	}

	// Simulation feature => time the verification process.
	if (VerifyResponse == 1 && pc.IsRoot()) || VerifyResponse == 2 {
		dbg.Lvl3(pc.Name(), "(root=", pc.IsRoot(), ") Doing Response verification", VerifyResponse)
		// verify the responses at each level with the aggregate public key of this
		// subtree.
		if err := pc.Cosi.VerifyResponses(pc.TreeNode().PublicAggregateSubTree); err != nil {
			dbg.Error("Verification error")
			return fmt.Errorf("%s Verifcation of responses failed:%s", pc.Name(), err)
		}
	} else {
		dbg.Lvl3(pc.Name(), "(root=", pc.IsRoot(), ") Skipping Response verification", VerifyResponse)
	}

	out := &Response{
		Response: outResponse,
	}
	// send it back to parent
	if !pc.IsRoot() {
		return pc.SendTo(pc.Parent(), out)
	}
	return nil
}

// Cleanup closes the protocol and calls DoneCallback, if defined
func (pc *ProtocolCosi) Cleanup() {
	dbg.Lvl3(pc.Entity().First(), "Cleaning up")
	// if callback when finished
	if pc.DoneCallback != nil {
		dbg.Lvl3("Calling doneCallback")
		pc.DoneCallback(pc.Cosi.GetChallenge(), pc.Cosi.GetAggregateResponse())
	}
	close(pc.done)
	pc.Done()

}

// SigningMessage simply set the message to sign for this round
func (pc *ProtocolCosi) SigningMessage(msg []byte) {
	pc.Message = msg
	dbg.Lvl2(pc.Name(), "Root will sign message=", pc.Message)
}

// RegisterAnnouncementHook allows for handling what should happen upon an
// announcement
func (pc *ProtocolCosi) RegisterAnnouncementHook(fn AnnouncementHook) {
	pc.announcementHook = fn
}

// RegisterCommitmentHook allows for handling what should happen when a
// commitment is received
func (pc *ProtocolCosi) RegisterCommitmentHook(fn CommitmentHook) {
	pc.commitmentHook = fn
}

// RegisterChallengeHook allows for handling what should happen when a
// challenge is received
func (pc *ProtocolCosi) RegisterChallengeHook(fn ChallengeHook) {
	pc.challengeHook = fn
}

// RegisterDoneCallback allows for handling what should happen when a
// the protocol is done
func (pc *ProtocolCosi) RegisterDoneCallback(fn func(chal, resp abstract.Secret)) {
	pc.DoneCallback = fn
}
