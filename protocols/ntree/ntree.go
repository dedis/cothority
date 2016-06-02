package ntree

import (
	"errors"
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
	sda.ProtocolRegisterName("NaiveTree", NewProtocol)
}

// Protocol implements the sda.ProtocolInstance interface
type Protocol struct {
	*sda.TreeNodeInstance
	// the message we want to sign (and the root node propagates)
	Message []byte
	// signature of this particular participant:
	signature *SignatureReply
	// Simulation related
	verifySignature int
}

// NewProtocol is used internally to register the protocol.
func NewProtocol(node *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
	p := &Protocol{
		TreeNodeInstance: node,
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

// Start implements sda.ProtocolInstance.
func (p *Protocol) Start() error {
	if p.IsRoot() {
		if len(p.Children()) > 0 {
			dbg.Lvl3("Starting ntree/naive")
			return p.HandleSignRequest(structMessage{p.TreeNode(),
				Message{p.Message, p.verifySignature}})
		} else {
			return errors.New("No children for root")
		}
	} else {
		return fmt.Errorf("Called Start() on non-root ProtocolInstance")
	}
}

// HandleSignRequest is a handler for incoming sign-requests. It's registered as
// a handler in the sda.Node.
func (p *Protocol) HandleSignRequest(msg structMessage) error {
	p.Message = msg.Msg
	p.verifySignature = msg.VerifySignature
	signature, err := crypto.SignSchnorr(network.Suite, p.Private(), p.Message)
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
		err := p.SendTo(p.Parent(), &SignatureBundle{ChildSig: p.signature})
		p.Done()
		return err
	}
	return nil
}

// HandleSignBundle is a handler responsible for adding the node's signature
// and verifying the children's signatures (verification level can be controlled
// by the VerifySignature flag).
func (p *Protocol) HandleSignBundle(reply []structSignatureBundle) {
	dbg.Lvl3("Appending our signature to the collected ones and send to parent")
	var sig SignatureBundle
	sig.ChildSig = p.signature
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
			s := p.verifySignatureReply(sigs.ChildSig)
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
		sig.SubSigs = append(sig.SubSigs, sigs.ChildSig)
		sig.SubSigs = append(sig.SubSigs, sigs.SubSigs...)
	}

	if !p.IsRoot() {
		p.SendTo(p.Parent(), &sig)
	} else {
		dbg.Lvl3("Leader got", len(reply), "signatures. Children:", len(p.Children()))
		p.Done()
	}
}

func (p *Protocol) verifySignatureReply(sig *SignatureReply) string {
	if sig.Index >= len(p.EntityList().List) {
		dbg.Error("Index in signature reply out of range")
		return "FAIL"
	}
	entity := p.EntityList().List[sig.Index]
	var s string
	if err := crypto.VerifySchnorr(p.Suite(), entity.Public, p.Message, sig.Sig); err != nil {
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
//   * SchnorrSig (signature of the current node)
//   * Index of the public key in the entitylist in order to verify the
//   signature
type SignatureReply struct {
	Sig   crypto.SchnorrSig
	Index int
}

// SignatureBundle represent the signature that one children will pass up to its
// parent. It contains:
//  * The signature reply of a direct child (sig + index)
//  * The whole set of signature reply of the child sub tree
type SignatureBundle struct {
	// Child signature
	ChildSig *SignatureReply
	// Child subtree signatures
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
