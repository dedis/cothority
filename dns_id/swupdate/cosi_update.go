// Package swupdate implements a round of a Collective Signing protocol (like .
package swupdate

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/dedis/onet/log"
	"github.com/dedis/onet"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/cosi"
)

// ProtocolName defines the name of this protocol.
const ProtocolName = "CoSiUpdate"

// CoSiUpdate protocol is a CoSi version with
// four phases:
//  - Announcement: The message is being passed into this pass down the tree
//  - Commitment: Each node have decided if they agree to sign or not and let
//  their parent know.
//  - Challenge: as in vanilla cosi
//  - Response: as in vanilla cosi
// When registering this protocol, you must give to the constructor of
// CoSiUpdate, a VerificationHook that will be called when the Announcement
// message is received with the proper embedded data.
// If the function returns true (the function is called in a goroutine),
// then the local node will participate in the signing, otherwise, he won't and
// will just have a role of relay.
type CoSiUpdate struct {
	// The node that represents us
	*onet.TreeNodeInstance
	// TreeNodeId cached
	treeNodeID onet.TreeNodeID
	// the cosi struct we use (since it is a cosi protocol)
	// Public because we will need it from other protocols.
	cosi *cosi.CoSi
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
	tempCommitment []abstract.Point
	// temp buffer of index of refusing-to-sign nodes
	tempRefusing []uint32
	// lock associated
	tempCommitLock *sync.Mutex
	// temporary buffer of Response messages
	tempResponse []abstract.Scalar
	// lock associated
	tempResponseLock *sync.Mutex

	// isSigningMut is the mutex for checking the isSigning bool var
	isSigningMut sync.Mutex
	// isSigning tells if this node is participating or not
	isSigning bool
	// verificationChan gives back the result from the verification function
	verificationChan chan bool
	// hook related to the verification of the message to sign
	verificationHook VerificationHook

	// hooks related to the various phase of the protocol.
	signatureHook SignatureHook
}

// VerificationHook is the function that receives the data to sign on
type VerificationHook func(data []byte) bool

// SignatureHook is the function that is called when the signature is ready
// (it's called on the root since only the root has the final signature)
type SignatureHook func(sig []byte)

// NewCoSiUpdate takes a verification function and a TreeNodeInstance and will
// return a fresh CoSiUpdate ProtocolInstance.
// Use this function like this:
// ```
// fn := func(n *onet.TreeNodeInstance) onet.ProtocolInstance {
//      pc := NewCoSiUpdate(n,myVerificationFunction)
//		return pc
// }
// onet.RegisterNewProtocolName("MyCoSiSoftwareUpdate",fn)
// ```
func NewCoSiUpdate(node *onet.TreeNodeInstance, fn VerificationHook) (*CoSiUpdate, error) {
	var err error
	// XXX just need to take care to take the global list of cosigners once we
	// do the exception stuff
	publics := make([]abstract.Point, len(node.Roster().List))
	for i, e := range node.Roster().List {
		publics[i] = e.Public
	}
	c := &CoSiUpdate{
		cosi:             cosi.NewCosi(node.Suite(), node.Private(), publics),
		TreeNodeInstance: node,
		done:             make(chan bool),
		tempCommitLock:   new(sync.Mutex),
		tempResponseLock: new(sync.Mutex),
		tempRefusing:     make([]uint32, 0), // in case there's no exception, protobuf fails otherwise
		verificationChan: make(chan bool),
		verificationHook: fn,
	}
	// Register the channels we want to register and listens on

	if err := node.RegisterChannel(&c.announce); err != nil {
		return c, err
	}
	if err := node.RegisterChannel(&c.commit); err != nil {
		return c, err
	}
	if err := node.RegisterChannel(&c.challenge); err != nil {
		return c, err
	}
	if err := node.RegisterChannel(&c.response); err != nil {
		return c, err
	}

	return c, err
}

// Dispatch will listen on the four channels we use (i.e. four steps)
func (c *CoSiUpdate) Dispatch() error {
	for {
		var err error
		select {
		case packet := <-c.announce:
			err = c.handleAnnouncement(&packet.Announcement)
		case packet := <-c.commit:
			err = c.handleCommitment(&packet.Commitment)
		case packet := <-c.challenge:
			err = c.handleChallenge(&packet.Challenge)
		case packet := <-c.response:
			err = c.handleResponse(&packet.Response)
		case <-c.done:
			return nil
		}
		if err != nil {
			log.Error("ProtocolCosi -> err treating incoming:", err)
		}
	}
}

// Start will call the announcement function of its inner Round structure. It
// will pass nil as *in* message.
func (c *CoSiUpdate) Start() error {
	out := &Announcement{Data: c.Message}
	return c.handleAnnouncement(out)
}

// VerifySignature verifies if the challenge and the secret (from the response phase) form a
// correct signature for this message using the aggregated public key.
// This is copied from cosi, so that you don't need to include both lib/cosi
// and protocols/cosi
func VerifySignature(suite abstract.Suite, publics []abstract.Point, msg, sig []byte) error {
	return cosi.VerifySignature(suite, publics, msg, sig)
}

// handleAnnouncement will pass the message to the round and send back the
// output. If in == nil, we are root and we start the round.
func (c *CoSiUpdate) handleAnnouncement(in *Announcement) error {
	//log.LLvl3("Message:", in.Data)
	if c.verificationHook != nil {
		// write to the channel when the verification function is done
		go func() {
			// the root returns true whatsoever
			// TODO relax that assumption later if needed. Way simpler like this
			// because the current version of dedis/crypto/cosi does not handle
			// the case where the root does not want to sign (it's the one
			// creating the challenge so it should not include its own commit)
			if c.IsRoot() {
				c.verificationChan <- true
			} else {
				c.verificationChan <- c.verificationHook(in.Data)
			}
		}()
	}

	// If we are leaf, we should go to commitment
	if c.IsLeaf() {
		return c.handleCommitment(nil)
	}
	// send to children
	return c.SendToChildren(in)
}

// handleAllCommitment relay the commitments up in the tree
// It expects *in* to be the full set of messages from the children.
// The children's commitment must remain constants.
func (c *CoSiUpdate) handleCommitment(in *Commitment) error {
	if !c.IsLeaf() {
		// add to temporary
		c.tempCommitLock.Lock()
		c.tempCommitment = append(c.tempCommitment, in.Comm)
		c.tempRefusing = append(c.tempRefusing, in.RefusingNodes...)
		c.tempCommitLock.Unlock()
		// do we have enough ?
		if len(c.tempCommitment) < len(c.Children()) {
			return nil
		}
	}
	log.Lvl3(c.Name(), "aggregated")

	// wait for the verification function to return
	c.isSigningMut.Lock()
	c.isSigning = <-c.verificationChan

	var out = c.Suite().Point().Null()
	var refusing []uint32
	// this node should not sign, so it just relays the commitment it received
	if c.isSigning {
		// go to Commit()
		out = c.cosi.Commit(nil, c.tempCommitment)
	} else {
		// aggregate all children's commitment and pass it up
		for _, p := range c.tempCommitment {
			out.Add(out, p)
		}
		refusing = append(refusing, uint32(c.Index()))
	}
	c.isSigningMut.Unlock()

	// if we are the root, we need to start the Challenge
	if c.IsRoot() {
		return c.startChallenge()
	}

	// otherwise send it to parent
	outMsg := &Commitment{
		Comm:          out,
		RefusingNodes: refusing,
	}
	return c.SendTo(c.Parent(), outMsg)
}

// StartChallenge starts the challenge phase. Typically called by the Root ;)
func (c *CoSiUpdate) startChallenge() error {
	// remove the non-participating nodes from the Challenge + responses phases
	var max = uint32(len(c.Roster().List))
	for _, idx := range c.tempRefusing {
		if idx > max {
			log.Lvl3("Error indexing a refusing node:", idx, "/", max)
			continue
		}
		c.cosi.SetMaskBit(int(idx), false)
	}

	challenge, err := c.cosi.CreateChallenge(c.Message)
	if err != nil {
		return err
	}
	out := &Challenge{
		Chall: challenge,
	}
	log.Lvl3(c.Name(), "Starting Chal=", fmt.Sprintf("%+v", challenge), " (message =", hex.EncodeToString(c.Message))
	return c.handleChallenge(out)

}

// handleChallenge dispatch the challenge to the round and then dispatch the
// results down the tree.
func (c *CoSiUpdate) handleChallenge(in *Challenge) error {
	log.Lvl3(c.Name(), "chal=", fmt.Sprintf("%+v", in.Chall))

	c.cosi.Challenge(in.Chall)

	// if we are leaf, then go to response
	if c.IsLeaf() {
		return c.handleResponse(nil)
	}

	// otherwise send it to children
	return c.SendToChildren(in)
}

// handleResponse brings up the response of each node in the tree to the root.
func (c *CoSiUpdate) handleResponse(in *Response) error {
	if !c.IsLeaf() {
		// add to temporary
		c.tempResponseLock.Lock()
		c.tempResponse = append(c.tempResponse, in.Resp)
		c.tempResponseLock.Unlock()
		// do we have enough ?
		log.Lvl3(c.Name(), "has", len(c.tempResponse), "responses")
		if len(c.tempResponse) < len(c.Children()) {
			return nil
		}
	}
	log.Lvl3(c.Name(), "aggregated all responses")

	defer func() {
		// protocol is finished
		close(c.done)
		c.Done()
	}()

	var out = c.Suite().Scalar().Zero()
	var err error
	c.isSigningMut.Lock()

	if c.isSigning {
		// we sign
		if out, err = c.cosi.Response(c.tempResponse); err != nil {
			return err
		}
	} else {
		// we just relay the responses
		for _, s := range c.tempResponse {
			out.Add(out, s)
		}
	}
	c.isSigningMut.Unlock()

	response := &Response{
		Resp: out,
	}

	// send it back to parent
	if !c.IsRoot() {
		return c.SendTo(c.Parent(), response)
	}

	// we are root, we have the signature now
	if c.signatureHook != nil {
		c.signatureHook(c.cosi.Signature())
	}
	return nil
}

// VerifyResponses allows to check at each intermediate node whether the
// responses are valid
func (c *CoSiUpdate) VerifyResponses(agg abstract.Point) error {
	return c.cosi.VerifyResponses(agg)
}

// SigningMessage simply set the message to sign for this round
func (c *CoSiUpdate) SigningMessage(msg []byte) {
	c.Message = msg
	log.Lvlf2(c.Name(), "Root will sign message %x", c.Message)
}

// RegisterSignatureHook allows for handling what should happen when
// the protocol is done
func (c *CoSiUpdate) RegisterSignatureHook(fn SignatureHook) {
	c.signatureHook = fn
}

// RegisterVerificationHook can be used to register a handler which will be
// called during the Announcement phase. It will be called on the message which
// is passed during the announcement phase.
func (c *CoSiUpdate) RegisterVerificationHook(fn VerificationHook) {
	c.verificationHook = fn
}
