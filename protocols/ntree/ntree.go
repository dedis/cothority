// Simple protocol where each node signs a message and the parent node verifies
// it.
// Implements a scheme where a leader (the root node) collects N standard individual signatures
// from the N witnesses using a tree. As the "naive" scheme where the leader directly sends the message to be signed
// directly to its children is a special case if ntree (a 1-level tree) this can also be used to measure how the naive
// approach compares to ntree and CoSi.
package ntree

import (
	"fmt"
	"github.com/dedis/cothority/lib/crypto"
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
)

func init() {
	// register network messages and protocol
	network.RegisterMessageType(Message{})
	network.RegisterMessageType(SignatureReply{})
}

// Protocol implements the sda.ProtocolInstance interface
type Protocol struct {
	*sda.Node
	// the message we want to sign (and the root node propagates)
	message []byte
	// signature of this particular participant:
	signature *SignatureReply
	// Simulation related
	verifySignature int
}

func NewProtocol(node *sda.Node) (*Protocol, error) {
	p := &Protocol{
		Node: node,
	}
	err := p.RegisterHandler(p.HandleSignRequest)
	if err != nil {
		return nil, fmt.Errorf("Couldn't register handler %v", err)
	}
	err = p.RegisterHandler(p.HandleSignBundle)
	if err != nil {
		return nil, fmt.Errorf("Couldn't register handler %v", err)
	}
	return p, nil
}

func (p *Protocol) Start() error {
	if p.IsRoot() {
		dbg.Lvl3("Starting ntree/naive")
		return p.HandleSignRequest(structMessage{p.TreeNode(),
			Message{p.message, p.verifySignature}})
	} else {
		return fmt.Errorf("Called Start() on non-root ProtocolInstance")
	}
}

func (p *Protocol) HandleSignRequest(msg structMessage) error {
	p.message = msg.Msg
	p.verifySignature = msg.VerifySignature
	signature, err := crypto.SignSchnorr(network.Suite, p.Private(), p.message)
	if err != nil {
		return err
	}
	// fill our own signature
	p.signature = &SignatureReply{
		Sig:   signature,
		Index: p.TreeNode().EntityIdx}
	if !p.IsLeaf() {
		for _, c := range p.Children() {
			err := p.SendTo(c, &msg.Message)
			if err != nil {
				return err
			}
		}
	} else {
		err := p.SendTo(p.Parent(), &SignatureBundle{OwnSig: p.signature})
		p.Done()
		return err
	}
	return nil
}

func (p *Protocol) HandleSignBundle(reply []structSignatureBundle) error {
	dbg.Lvl3("Appending our signature to the collected ones and send to parent")
	var sig SignatureBundle
	sig.OwnSig = p.signature
	// at least n signature from direct children
	count := len(reply)
	for _, s := range reply {
		// and count how many from the sub-trees
		count += len(s.SubSigs)
	}
	sig.SubSigs = make([]*SignatureReply, count)
	for _, sigs := range reply {
		// Check only direct children
		// see https://github.com/dedis/cothority/issues/260
		if p.verifySignature == 1 || p.verifySignature == 2 {
			s := p.verifySignatureReply(sigs.OwnSig)
			dbg.Lvl3(p.Name(), "direct children verification:", s)
		}
		// Verify also the whole subtree
		if p.verifySignature == 2 {
			dbg.Lvl3(p.Name(), "Doing Subtree verification")
			for _, sub := range sigs.SubSigs {
				s := p.verifySignatureReply(sub)
				dbg.Lvl3(p.Name(), "verifying subtree signature:", s)
			}
		}
		if p.verifySignature == 0 {
			dbg.Lvl3(p.Name(), "Skipping signature verification..")
		}
		// add both the children signature + the sub tree signatures
		sig.SubSigs = append(sig.SubSigs, sigs.OwnSig)
		sig.SubSigs = append(sig.SubSigs, sigs.SubSigs...)
	}§§§

	if !p.IsRoot() {
		return p.SendTo(p.Parent(), &sig)
	}

	dbg.Lvl3("Leader got", len(reply), "signatures. Children:", len(p.Children()))
	p.Done()
	return nil
}

func (p *Protocol) SetMessage(msg []byte) {
	p.message = msg
}

func (p *Protocol) verifySignatureReply(sig *SignatureReply) string {
	if sig.Index >= len(p.EntityList().List) {
		dbg.Error("Index in signature reply out of range")
		return "FAIL"
	}
	entity := p.EntityList().List[sig.Index]
	var s string
	if err := crypto.VerifySchnorr(p.Suite(), entity.Public, p.message, sig.Sig); err != nil {
		s = "FAIL"
	} else {
		s = "SUCCESS"
	}
	return s
}

// ----- network messages that will be sent around ------- //

// Message contains the actual message (as a slice of bytes) that will be individually signed
type Message struct {
	Msg []byte
	// Simulation purpose
	// see https://github.com/dedis/cothority/issues/260
	VerifySignature int
}

// SignatureReply contains a signature for the message
//   * SchnorrSig (signature of the current node + those of its children)
//   * Index of the public key in the entitylist in order to verify the
//   signature
type SignatureReply struct {
	Sig   crypto.SchnorrSig
	Index int
}

// SignatureBundle represent the signature that one children will pass up to its
// parent. It contains:
//  * The signature reply of this children (sig + index)
//  * The whole set of signature reply of its sub tree
type SignatureBundle struct {
	OwnSig  *SignatureReply
	SubSigs []*SignatureReply
}

type structMessage struct {
	*sda.TreeNode
	Message
}

type structSignatureBundle struct {
	*sda.TreeNode
	SignatureBundle
}

// ---------------- end: network messages  --------------- //
