package jvss

import (
	"github.com/dedis/cothority/lib/sda"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

func init() {
	sda.ProtocolRegisterName("JVSS", NewJVSS)
}

// JVSS is the main protocol struct and implements the sda.ProtocolInstance
// interface.
type JVSS struct {
	*sda.Node                   // The SDA TreeNode
	idx       int               // Index of the node in the EntityList
	keyPair   config.KeyPair    // KeyPair of the host
	nodeList  []*sda.TreeNode   // List of TreeNodes
	pKeys     []*abstract.Point // List of public keys of the TreeNodes
}

// NewJVSS creates a new JVSS protocol instance and returns it.
func NewJVSS(node *sda.Node) (sda.ProtocolInstance, error) {

	kp := config.KeyPair{Suite: node.Suite(), Public: node.Public(), Secret: node.Private()}
	nodes := node.Tree().ListNodes()
	pk := make([]*abstract.Point, len(nodes))

	for i, tn := range nodes {

	}

	jvss := &JVSS{
		idx:     node.TreeNode().EntityIdx,
		keyPair: kp,
	}

	return jvss, nil
}

// Start initiates the JVSS protocol.
func (jv *JVSS) Start() error {

}

func (jv *JVSS) nodeIdx() int {
	return jv.Node.TreeNode().EntityIdx
}
