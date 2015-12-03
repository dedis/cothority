package sign

import (
	"github.com/dedis/cothority/lib/coconet"
	"github.com/dedis/crypto/abstract"
)

/*
This structure holds basic information about a round. It
can be included in a structure. To initialise, the
round has to call NewRoundStruct.
*/

type RoundStruct struct {
	Node     *Node
	Type     string
	Name     string
	IsRoot   bool
	IsLeaf   bool
	RoundNbr int
	ViewNbr  int
	Parent   string
	Children map[string]coconet.Conn
	Suite    abstract.Suite
}

func NewRoundStruct(node *Node, rtype string) *RoundStruct {
	viewNbr := node.ViewNo
	roundNbr := node.nRounds
	children := node.Children(viewNbr)
	cbs := &RoundStruct{
		Node:     node,
		Type:     rtype,
		Name:     node.Name(),
		IsRoot:   node.IsRoot(viewNbr),
		IsLeaf:   len(children) == 0,
		RoundNbr: roundNbr,
		ViewNbr:  viewNbr,
		Parent:   node.Parent(viewNbr),
		Children: children,
		Suite:    node.Suite(),
	}
	return cbs
}

func (r *RoundStruct) GetType() string {
	return r.Type
}
