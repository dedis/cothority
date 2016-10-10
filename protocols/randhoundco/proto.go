package proto

import (
	"github.com/dedis/cothority/protocols/jvss"
	"github.com/dedis/cothority/sda"
)

const ProtoName = "RandhounCo"

func init() {
	sda.ProtocolRegisterName(ProtoName, NewRandhounCo)
}

// RandhoundCo holds all informations to run a round of a JVSS-based CoSi.
// Basically, each node on the tree, except the root, represents a JVSS group
// with other
type RandhoundCo struct {
}

// NewRandhoundCoRoot returns a protocol instance which is used by the root of
// the tree.
func NewRandhoundCoRoot(n *sda.TreeNode) (sda.ProtocolInstance, error) {

}

func NewRandhoundCoNode(n *sda.TreeNode, jvss *jvss.JVSS) (sda.ProtocolInstance, error) {

}
