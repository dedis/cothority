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
	Name     string
	IsRoot   bool
	IsLeaf   bool
	RoundNbr int
	ViewNbr  int
	Parent   string
	Children map[string]coconet.Conn
	Suite    abstract.Suite
}

func NewRoundStruct(node *Node) *RoundStruct {
	viewNbr := node.ViewNo
	roundNbr := node.nRounds
	children := node.Children(viewNbr)
	cbs := &RoundStruct{
		Node: node,
		Name: node.Name(),
		IsRoot: node.IsRoot(viewNbr),
		IsLeaf: len(children) == 0,
		RoundNbr: roundNbr,
		ViewNbr: viewNbr,
		Parent: node.Parent(viewNbr),
		Children: children,
		Suite: node.Suite(),
	}
	return cbs
}

func (r *RoundStruct)SetRoundType(roundType string, out []*SigningMessage) {
	for i := range (out) {
		out[i].Am.RoundType = roundType
	}
}