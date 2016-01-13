package sign

import (
	"github.com/dedis/cothority/lib/tree"
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
	Children []*tree.Node
	//	Children map[string]coconet.Conn
	Suite abstract.Suite
}

func NewRoundStruct(node *Node, rtype string) *RoundStruct {
	viewNbr := node.ViewNo
	roundNbr := node.nRounds
	children := node.Children(viewNbr)
	var parent string
	if pNode := node.Parent(viewNbr); pNode == nil {
		parent = ""
	} else {
		parent = pNode.Name()
	}
	cbs := &RoundStruct{
		Node:     node,
		Type:     rtype,
		Name:     node.Name(),
		IsRoot:   node.Root(viewNbr),
		IsLeaf:   len(children) == 0,
		RoundNbr: roundNbr,
		ViewNbr:  viewNbr,
		Parent:   parent,
		//		Children: children,
		Suite: node.Suite(),
	}
	return cbs
}

func (r *RoundStruct) GetType() string {
	return r.Type
}
