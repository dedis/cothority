// Package ntree implements a scheme where a leader (the root node) collects N standard individual signatures
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
	signature crypto.SchnorrSig
}

func NewProtocol(node *sda.Node) (*Protocol, error) {
	p := &Protocol{
		Node: node,
	}
	err := p.RegisterHandler(p.HandleSignRequest)
	if err != nil {
		return nil, fmt.Errorf("Couldn't register handler %v", err)
	}
	err = p.RegisterHandler(p.HandleSignReply)
	if err != nil {
		return nil, fmt.Errorf("Couldn't register handler %v", err)
	}
	return p, nil
}

func NewRootProtocol(msg []byte, node *sda.Node) (*Protocol, error) {
	p, err := NewProtocol(node)
	if err != nil {
		return nil, err
	}
	p.message = msg
	return p, err
}

func (p *Protocol) Start() error {
	if p.IsRoot() {
		dbg.Lvl3("Starting ntree/naive")
		return p.HandleSignRequest(structMessage{p.TreeNode(), Message{p.message}})
	} else {
		return fmt.Errorf("Called Start() on non-root ProtocolInstance")
	}
}

func (p *Protocol) HandleSignRequest(msg structMessage) error {
	var err error
	p.signature, err = crypto.SignSchnorr(network.Suite, p.Private(), p.message)
	if err != nil {
		return err
	}
	if !p.IsLeaf() {
		for _, c := range p.Children() {
			err := p.SendTo(c, &msg.Message)
			if err != nil {
				return err
			}
		}
	} else {
		return p.SendTo(p.Parent(), &SignatureReply{Signatures: []crypto.SchnorrSig{p.signature}})
	}
	return nil
}

func (p *Protocol) HandleSignReply(reply []structSignatureReply) error {
	if !p.IsRoot() {
		dbg.Lvl3("Appending our signature to the collected ones and send to parent")
		count := 0
		for _, s := range reply {
			count += len(s.Signatures)
		}
		aggSignatures := make([]crypto.SchnorrSig, count+1)
		for i, sigs := range reply {
			for j, sig := range sigs.Signatures {
				aggSignatures[i+j] = sig
			}
		}
		aggSignatures[count] = p.signature
		return p.SendTo(p.Parent(), &SignatureReply{Signatures: aggSignatures})
	} else {
		dbg.Lvl3("Leader got", len(reply), "signatures. Children:", len(p.Children()))
	}
	return nil
}

// ----- network messages that will be sent around ------- //

// Message contains the actual message (as a slice of bytes) that will be individually signed
type Message struct {
	Msg []byte
}

// SignatureReply contains the signatures for the message (signature of the current node + those of its children)
type SignatureReply struct {
	Signatures []crypto.SchnorrSig
}

type structMessage struct {
	*sda.TreeNode
	Message
}

type structSignatureReply struct {
	*sda.TreeNode
	SignatureReply
}

// ---------------- end: network messages  --------------- //
